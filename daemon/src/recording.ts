// RecordingManager — CDP screencast recording piped into ffmpeg.
//
// Ports the agent-browser 0.37 recording contract: `record start` captures the
// current active page through `Page.startScreencast` on its own CDP session,
// the requested rate (1-60, default 30) drives a wall-clock ticker so the file
// duration matches the automation it recorded, and ffmpeg picks the container
// from the output extension. Missing ffmpeg, an extensionless path, or an
// out-of-range rate fail before a browser session, an encoder process, or a
// file is created.

import { spawn, type ChildProcessByStdio } from 'node:child_process';
import type { Readable, Writable } from 'node:stream';
import type { CDPSession, Page } from 'playwright-core';

/** ffmpeg with a writable stdin and a readable stderr, no stdout. */
type FfmpegProcess = ChildProcessByStdio<Writable, null, Readable>;

/** Capture rate used when the caller does not ask for one. */
export const DEFAULT_FPS = 30;

/** Highest capture rate the recorder accepts. */
export const MAX_FPS = 60;

/** Lowest capture rate the recorder accepts. */
export const MIN_FPS = 1;

/** Rate above which the encoder switches to its high-frame-rate profile. */
const HIGH_FPS_THRESHOLD = 30;

/** Bitrate budget for WebM at [`HIGH_FPS_THRESHOLD`], scaled with the rate. */
const WEBM_BITRATE_KBPS_AT_BASE_FPS = 1000;

/** Longest gap the recorder fills with held frames, in seconds. */
const MAX_BACKFILL_SECS = 5;

/** Screencast frames buffered ahead of the ticker. */
const MAX_PENDING_FRAMES = 2;

/** JPEG quality requested from `Page.startScreencast`. */
const SCREENCAST_QUALITY = 80;

/** Characters of ffmpeg stderr kept in a failure message. */
const FFMPEG_STDERR_TAIL_CHARS = 400;

/** Upper bound on waiting for ffmpeg to finalize a stopped recording. */
const FFMPEG_STOP_TIMEOUT_MS = 5000;

/** Bytes buffered toward ffmpeg before the ticker holds frames back. */
const MAX_STDIN_BUFFER_BYTES = 32 * 1024 * 1024;

export interface RecordingResult {
  path: string;
  frames: number;
  capturedFrames: number;
  fps: number;
  durationMs: number;
  bytes: number;
}

export interface RecordingStatus {
  active: boolean;
  path: string | null;
  fps: number | null;
  frames: number;
  capturedFrames: number;
  startedAt: number | null;
}

/** Lower-cased extension of `path`, if it has one. */
export function outputExtension(path: string): string | null {
  const match = /\.([^.\\/]+)$/.exec(path);
  return match ? match[1].toLowerCase() : null;
}

/**
 * Reject output paths with no extension. ffmpeg picks the container from the
 * extension, so a bare name fails at `record stop` with the take already lost.
 * `.webm` gets libvpx; every other extension gets libx264 in whatever container
 * ffmpeg knows for it.
 */
export function validateRecordingPath(path: string): void {
  if (!outputExtension(path)) {
    throw new Error(`Invalid output path: '${path}' has no extension (use .webm or .mp4)`);
  }
}

/** Reject frame rates the pipeline cannot honor. */
export function validateRecordingFps(fps: number): number {
  if (!Number.isInteger(fps) || fps < MIN_FPS || fps > MAX_FPS) {
    throw new Error(`Invalid fps: ${fps} is out of range (valid range: ${MIN_FPS}-${MAX_FPS})`);
  }
  return fps;
}

/** ffmpeg arguments for a take, matching the agent-browser encoder profile. */
export function buildFfmpegArgs(outputPath: string, fps: number): string[] {
  const highFps = fps > HIGH_FPS_THRESHOLD;
  const args = [
    '-y',
    '-hide_banner',
    '-avioflags', 'direct',
    '-fpsprobesize', '0',
    '-probesize', '1024',
    '-analyzeduration', '0',
    '-f', 'image2pipe',
    '-c:v', 'mjpeg',
    '-framerate', String(fps),
    '-i', 'pipe:0',
    '-vf', 'pad=ceil(iw/2)*2:ceil(ih/2)*2',
  ];

  if (outputExtension(outputPath) === 'webm') {
    const bitrate = Math.max(
      WEBM_BITRATE_KBPS_AT_BASE_FPS,
      Math.floor((WEBM_BITRATE_KBPS_AT_BASE_FPS * fps) / Math.max(1, HIGH_FPS_THRESHOLD)),
    );
    args.push('-c:v', 'libvpx', '-crf', '30', '-b:v', `${bitrate}k`);
  } else {
    args.push('-c:v', 'libx264', '-preset', 'ultrafast');
  }

  // One encoder thread keeps CPU away from the browser at ordinary rates;
  // above 30 fps the encoder needs a second one to drain the pipe in time.
  args.push('-pix_fmt', 'yuv420p', '-threads', highFps ? '2' : '1', outputPath);
  return args;
}

/** The last [`FFMPEG_STDERR_TAIL_CHARS`] of stderr, cut at a line boundary. */
export function ffmpegErrorTail(stderr: string): string {
  const trimmed = stderr.trimEnd();
  if (trimmed.length <= FFMPEG_STDERR_TAIL_CHARS) {
    return trimmed;
  }
  const tail = trimmed.slice(trimmed.length - FFMPEG_STDERR_TAIL_CHARS);
  const cut = tail.search(/[\r\n]/);
  return cut >= 0 && cut + 1 < tail.length ? tail.slice(cut + 1).trimStart() : tail;
}

/** Actionable failure text for a missing or broken ffmpeg binary. */
export function ffmpegLaunchError(error: unknown): string {
  const message = error instanceof Error ? error.message : String(error);
  return `ffmpeg not found or failed to execute: ${message}. Install ffmpeg to enable recording.`;
}

/** Frames owed to the constant-rate stream, given frames already written. */
export function framesDue(elapsedMs: number, periodMs: number, written: number): number {
  const period = Math.max(1, periodMs);
  const slot = Math.floor(elapsedMs / period);
  return Math.max(0, slot + 1 - written);
}

/** Verify ffmpeg can run without opening or modifying the destination. */
export async function checkFfmpegAvailable(): Promise<void> {
  await new Promise<void>((resolve, reject) => {
    let child;
    try {
      child = spawn('ffmpeg', ['-version'], { stdio: ['ignore', 'ignore', 'ignore'] });
    } catch (error) {
      reject(new Error(ffmpegLaunchError(error)));
      return;
    }
    child.once('error', (error) => reject(new Error(ffmpegLaunchError(error))));
    child.once('close', (code) => {
      if (code === 0) {
        resolve();
        return;
      }
      reject(new Error(ffmpegLaunchError(new Error(`ffmpeg -version exited with code ${code}`))));
    });
  });
}

export class RecordingManager {
  private active = false;
  private outputPath = '';
  private fps = DEFAULT_FPS;
  private framesWritten = 0;
  private capturedFrames = 0;
  private startedAt: number | null = null;
  private bytesWritten = 0;
  private session: CDPSession | null = null;
  private ffmpeg: FfmpegProcess | null = null;
  private ticker: NodeJS.Timeout | null = null;
  private pending: Buffer[] = [];
  private lastFrame: Buffer | null = null;
  private stderr = '';
  private backpressure = false;
  private exitCode: number | null | undefined;

  status(): RecordingStatus {
    return {
      active: this.active,
      path: this.active ? this.outputPath : null,
      fps: this.active ? this.fps : null,
      frames: this.framesWritten,
      capturedFrames: this.capturedFrames,
      startedAt: this.active ? this.startedAt : null,
    };
  }

  /**
   * Start a take against `page`. Validation and the ffmpeg check run before any
   * browser or encoder state changes, so a rejected request leaves the previous
   * take (if any) untouched.
   */
  async start(page: Page, path: string, fps?: number): Promise<RecordingStatus> {
    if (this.active) {
      throw new Error('Recording already active');
    }
    validateRecordingPath(path);
    const rate = validateRecordingFps(fps ?? DEFAULT_FPS);
    await checkFfmpegAvailable();

    const session = await page.context().newCDPSession(page);
    let ffmpeg: FfmpegProcess | null = null;
    try {
      this.capturedFrames = 0;
      this.framesWritten = 0;
      this.bytesWritten = 0;
      this.pending = [];
      this.lastFrame = null;
      this.stderr = '';
      this.backpressure = false;
      this.exitCode = undefined;

      session.on('Page.screencastFrame', (params: { data: string; sessionId: number }) => {
        void session
          .send('Page.screencastFrameAck', { sessionId: params.sessionId })
          .catch(() => {});
        this.capturedFrames += 1;
        const frame = Buffer.from(params.data, 'base64');
        this.pending.push(frame);
        while (this.pending.length > MAX_PENDING_FRAMES) {
          this.pending.shift();
        }
      });

      await session.send('Page.startScreencast', {
        format: 'jpeg',
        quality: SCREENCAST_QUALITY,
        // Always 1: Chrome skips frames by count, and a static page produces
        // exactly one, which a higher value would drop.
        everyNthFrame: 1,
      });

      ffmpeg = spawn('ffmpeg', buildFfmpegArgs(path, rate), {
        stdio: ['pipe', 'ignore', 'pipe'],
      });
      ffmpeg.stderr.on('data', (chunk: Buffer) => {
        this.stderr += chunk.toString('utf-8');
        if (this.stderr.length > FFMPEG_STDERR_TAIL_CHARS * 4) {
          this.stderr = this.stderr.slice(this.stderr.length - FFMPEG_STDERR_TAIL_CHARS * 4);
        }
      });
      ffmpeg.once('error', (error) => {
        this.stderr += ffmpegLaunchError(error);
      });
      ffmpeg.once('close', (code) => {
        this.exitCode = code;
      });
      ffmpeg.stdin.on('drain', () => {
        this.backpressure = false;
      });

      this.session = session;
      this.ffmpeg = ffmpeg;
      this.outputPath = path;
      this.fps = rate;
      this.startedAt = Date.now();
      this.active = true;
      this.startTicker();
      return this.status();
    } catch (error) {
      this.active = false;
      this.stopTicker();
      await session.detach().catch(() => {});
      if (ffmpeg) {
        ffmpeg.stdin.destroy();
        ffmpeg.kill('SIGKILL');
      }
      throw error instanceof Error ? error : new Error(String(error));
    }
  }

  /** Stop the active take and finalize its file. */
  async stop(): Promise<RecordingResult> {
    if (!this.active) {
      throw new Error('No recording is active');
    }

    this.stopTicker();
    const session = this.session;
    const ffmpeg = this.ffmpeg;
    const path = this.outputPath;
    const fps = this.fps;
    const frames = this.framesWritten;
    const capturedFrames = this.capturedFrames;
    const durationMs = this.startedAt === null ? 0 : Date.now() - this.startedAt;
    const stderr = this.stderr;

    this.active = false;
    this.session = null;
    this.ffmpeg = null;
    this.startedAt = null;

    if (session) {
      try {
        await session.send('Page.stopScreencast').catch(() => {});
      } finally {
        await session.detach().catch(() => {});
      }
    }

    let exitCode: number | null = 0;
    if (ffmpeg) {
      exitCode = await this.finalizeFfmpeg(ffmpeg);
    }

    // An empty take is the root cause worth naming: ffmpeg also fails on it,
    // but "no frames captured" is the actionable message.
    if (frames === 0) {
      throw new Error('No frames captured');
    }
    if (exitCode !== 0 && exitCode !== null) {
      throw new Error(`ffmpeg exited with code ${exitCode}: ${ffmpegErrorTail(stderr || this.stderr)}`);
    }

    return { path, frames, capturedFrames, fps, durationMs, bytes: this.bytesWritten };
  }

  /**
   * Replace the active take with a new one. A failed start leaves the previous
   * file on disk and reports the replacement error.
   */
  async restart(
    page: Page,
    path: string,
    fps?: number,
  ): Promise<{ previousPath: string | null; status: RecordingStatus }> {
    let previousPath: string | null = null;
    if (this.active) {
      previousPath = (await this.stop()).path;
    }
    const started = await this.start(page, path, fps);
    return { previousPath, status: started };
  }

  /** Stop any active take; used on browser teardown. Best effort. */
  async shutdown(): Promise<void> {
    if (!this.active) {
      return;
    }
    try {
      await this.stop();
    } catch {
      // The browser may already be gone; nothing else to release.
    }
  }

  // ── Internals ─────────────────────────────────────────────────────────────

  private startTicker(): void {
    const periodMs = Math.max(1, Math.round(1000 / this.fps));
    const maxFramesPerTick = MAX_BACKFILL_SECS * this.fps + 1;
    this.ticker = setInterval(() => {
      if (this.startedAt === null || !this.ffmpeg) {
        return;
      }
      const due = framesDue(Date.now() - this.startedAt, periodMs, this.framesWritten);
      if (due <= 0) {
        return;
      }
      const limit = Math.min(due, maxFramesPerTick);
      for (let i = 0; i < limit; i += 1) {
        const frame = this.pending.shift() ?? this.lastFrame;
        if (!frame) {
          return;
        }
        this.writeFrame(frame);
      }
    }, periodMs);
    this.ticker.unref?.();
  }

  private stopTicker(): void {
    if (this.ticker) {
      clearInterval(this.ticker);
      this.ticker = null;
    }
  }

  private writeFrame(frame: Buffer): void {
    const stdin = this.ffmpeg?.stdin;
    if (!stdin || stdin.destroyed) {
      return;
    }
    if (this.backpressure && stdin.writableLength > MAX_STDIN_BUFFER_BYTES) {
      return;
    }
    this.lastFrame = frame;
    const accepted = stdin.write(frame);
    this.backpressure = !accepted;
    this.framesWritten += 1;
    this.bytesWritten += frame.length;
  }

  private async finalizeFfmpeg(ffmpeg: FfmpegProcess): Promise<number | null> {
    const exited = new Promise<number | null>((resolve) => {
      if (this.exitCode !== undefined) {
        resolve(this.exitCode);
        return;
      }
      ffmpeg.once('close', (code) => resolve(code));
    });

    ffmpeg.stdin.end();

    const timeout = new Promise<'timeout'>((resolve) => {
      const handle = setTimeout(() => resolve('timeout'), FFMPEG_STOP_TIMEOUT_MS);
      handle.unref?.();
    });

    const result = await Promise.race([exited, timeout]);
    if (result === 'timeout') {
      ffmpeg.kill('SIGKILL');
      return await exited;
    }
    return result;
  }
}
