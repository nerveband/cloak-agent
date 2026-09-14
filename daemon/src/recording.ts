// CDP screencast recording with transactional output and exact-rate frame timing.
import { spawn, type ChildProcessByStdio } from 'node:child_process';
import { mkdtemp, rename, rm, stat } from 'node:fs/promises';
import { basename, dirname, join, resolve } from 'node:path';
import { performance } from 'node:perf_hooks';
import type { Readable, Writable } from 'node:stream';
import type { CDPSession, Page } from 'playwright-core';

type Encoder = ChildProcessByStdio<Writable, null, Readable>;
export const DEFAULT_FPS = 30;
export const MAX_FPS = 60;
export const MIN_FPS = 1;
const TIMEOUT_MS = 5000;
const MAX_BUFFER_BYTES = 32 * 1024 * 1024;

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
  error?: string;
}
interface Destination { path: string; directory: string; temporary: string; fps: number }
interface Take {
  destination: Destination;
  session: CDPSession;
  encoder: Encoder;
  exited: Promise<{ code: number | null; signal: string | null }>;
  timer?: NodeJS.Timeout;
  epoch: number;
  started: number;
  frames: number;
  captured: number;
  frame?: Buffer;
  stderr: string;
  failure?: Error;
  stopping: boolean;
}

export function outputExtension(path: string): string | null {
  return /\.([^.\\/]+)$/.exec(path)?.[1].toLowerCase() ?? null;
}
export function validateRecordingPath(path: string): void {
  if (!outputExtension(path)) throw new Error(`Invalid output path: '${path}' has no extension (use .webm or .mp4)`);
}
export function validateRecordingFps(fps: number): number {
  if (!Number.isInteger(fps) || fps < MIN_FPS || fps > MAX_FPS) {
    throw new Error(`Invalid fps: ${fps} is out of range (valid range: ${MIN_FPS}-${MAX_FPS})`);
  }
  return fps;
}
export function buildFfmpegArgs(path: string, fps: number): string[] {
  const args = ['-y', '-hide_banner', '-loglevel', 'error', '-f', 'image2pipe', '-c:v', 'mjpeg',
    '-framerate', String(fps), '-i', 'pipe:0', '-vf', 'pad=ceil(iw/2)*2:ceil(ih/2)*2'];
  if (outputExtension(path) === 'webm') {
    args.push('-c:v', 'libvpx', '-crf', '30', '-b:v', `${Math.max(1000, Math.floor(1000 * fps / 30))}k`);
  } else {
    args.push('-c:v', 'libx264', '-preset', 'ultrafast');
  }
  return [...args, '-pix_fmt', 'yuv420p', '-threads', fps > 30 ? '2' : '1', path];
}
export function ffmpegErrorTail(stderr: string): string { return stderr.trim().slice(-400); }
export function ffmpegLaunchError(error: unknown): string {
  return `ffmpeg not found or failed to execute: ${error instanceof Error ? error.message : error}. Install ffmpeg to enable recording.`;
}
// The timer is only a wakeup mechanism. Never round the frame-period arithmetic.
export function framesDue(elapsedMs: number, fps: number, written: number): number {
  return Math.max(0, Math.floor(elapsedMs * fps / 1000) - written);
}
async function bounded<T>(operation: Promise<T>, label: string): Promise<T> {
  let timer: NodeJS.Timeout | undefined;
  try {
    return await Promise.race([operation, new Promise<never>((_, reject) => {
      timer = setTimeout(() => reject(new Error(`${label} timed out`)), TIMEOUT_MS);
    })]);
  } finally { clearTimeout(timer); }
}

export class RecordingManager {
  private take: Take | null = null;
  private busy = false;

  status(): RecordingStatus {
    const take = this.take;
    return {
      active: take !== null, path: take?.destination.path ?? null, fps: take?.destination.fps ?? null,
      frames: take?.frames ?? 0, capturedFrames: take?.captured ?? 0, startedAt: take?.epoch ?? null,
      ...(take?.failure ? { error: take.failure.message } : {}),
    };
  }

  private async exclusive<T>(operation: () => Promise<T>): Promise<T> {
    if (this.busy) throw new Error('Recording operation already in progress');
    this.busy = true;
    try { return await operation(); } finally { this.busy = false; }
  }

  private async prepare(path: string, fps = DEFAULT_FPS): Promise<Destination> {
    validateRecordingPath(path);
    validateRecordingFps(fps);
    path = resolve(path);
    if (this.take?.destination.path === path) throw new Error('A replacement recording must use a different output path');
    const existing = await stat(path).catch((error: NodeJS.ErrnoException) => {
      if (error.code !== 'ENOENT') throw error;
      return null;
    });
    if (existing && !existing.isFile()) throw new Error('Recording destination must be a regular file');
    const directory = await mkdtemp(join(dirname(path), '.cloak-record-'));
    const temporary = join(directory, basename(path));
    const encoderArgs = buildFfmpegArgs(temporary, fps);
    const probe = spawn('ffmpeg', ['-y', '-hide_banner', '-loglevel', 'error', '-f', 'lavfi', '-i',
      'color=c=black:s=2x2:r=1', '-frames:v', '1', ...encoderArgs.slice(encoderArgs.indexOf('-vf'))],
      { stdio: ['ignore', 'ignore', 'pipe'] });
    const probeClosed = new Promise<void>(resolve => probe.once('close', () => resolve()));
    let detail = '';
    probe.stderr.on('data', (data: Buffer) => { detail = (detail + data.toString()).slice(-400); });
    try {
      await bounded(new Promise<void>((resolve, reject) => {
        probe.once('error', error => reject(new Error(ffmpegLaunchError(error))));
        probe.once('close', (code, signal) => code === 0 ? resolve() : reject(new Error(`Recording output preflight failed: ${signal ?? code}: ${detail}`)));
      }), 'Recording output preflight');
      return { path, directory, temporary, fps };
    } catch (error) {
      probe.kill('SIGKILL');
      await bounded(probeClosed, 'Recording preflight cleanup').catch(() => {});
      await rm(directory, { recursive: true, force: true });
      throw error;
    }
  }

  async start(page: Page, path: string, fps?: number): Promise<RecordingStatus> {
    return this.exclusive(async () => {
      if (this.take) throw new Error('Recording already active');
      return this.begin(page, await this.prepare(path, fps));
    });
  }

  private async begin(page: Page, destination: Destination): Promise<RecordingStatus> {
    let session: CDPSession | undefined;
    let take: Take | undefined;
    try {
      session = await bounded(page.context().newCDPSession(page), 'CDP recording attach');
      const encoder = spawn('ffmpeg', buildFfmpegArgs(destination.temporary, destination.fps), { stdio: ['pipe', 'ignore', 'pipe'] });
      const exited = new Promise<{ code: number | null; signal: string | null }>((resolve) => {
        encoder.once('close', (code, signal) => resolve({ code, signal }));
      });
      take = { destination, session, encoder, exited, epoch: Date.now(), started: performance.now(),
        frames: 0, captured: 0, stderr: '', stopping: false };
      const current = take;
      const fail = (error: Error) => {
        current.failure ??= error;
        clearInterval(current.timer);
      };
      encoder.on('error', error => fail(new Error(ffmpegLaunchError(error))));
      encoder.stdin.on('error', error => fail(new Error(`ffmpeg input failed: ${error.message}`)));
      encoder.stderr.on('data', (data: Buffer) => { current.stderr = (current.stderr + data.toString()).slice(-1600); });
      void exited.then(({ code, signal }) => {
        if (!current.stopping || code !== 0) fail(new Error(`ffmpeg exited: ${signal ?? code}: ${ffmpegErrorTail(current.stderr)}`));
      });
      await bounded(new Promise<void>((resolve, reject) => {
        encoder.once('spawn', resolve);
        encoder.once('error', reject);
      }), 'ffmpeg start');
      session.on('Page.screencastFrame', (event: { data: string; sessionId: number }) => {
        void current.session.send('Page.screencastFrameAck', { sessionId: event.sessionId }).catch(() => {});
        current.captured++;
        if (event.data.length > MAX_BUFFER_BYTES * 4 / 3) {
          fail(new Error('Recording frame exceeds buffer limit'));
          return;
        }
        current.frame = Buffer.from(event.data, 'base64');
      });
      await bounded(session.send('Page.startScreencast', { format: 'jpeg', quality: 80, everyNthFrame: 1 }), 'CDP recording start');
      if (current.failure) throw current.failure;
      current.epoch = Date.now();
      current.started = performance.now();
      current.timer = setInterval(() => this.tick(current), Math.max(1, Math.floor(1000 / destination.fps)));
      current.timer.unref();
      this.take = current;
      return this.status();
    } catch (error) {
      if (take) {
        clearInterval(take.timer);
        take.encoder.stdin.destroy();
        take.encoder.kill('SIGKILL');
        await bounded(take.exited, 'ffmpeg cleanup').catch(() => {});
      }
      if (session) await bounded(session.detach(), 'CDP recording detach').catch(() => {});
      await rm(destination.directory, { recursive: true, force: true });
      throw error;
    }
  }

  private tick(take: Take): void {
    if (take.failure || !take.frame) return;
    const due = framesDue(performance.now() - take.started, take.destination.fps, take.frames);
    if (due > take.destination.fps * 5 || take.encoder.stdin.writableLength + take.frame.length * due > MAX_BUFFER_BYTES) {
      take.failure = new Error('Recording encoder cannot keep up with the requested frame rate');
      clearInterval(take.timer);
      return;
    }
    if (take.encoder.stdin.destroyed) {
      take.failure = new Error('Recording encoder input closed unexpectedly');
      clearInterval(take.timer);
      return;
    }
    for (let i = 0; i < due; i++) {
      take.encoder.stdin.write(take.frame);
      take.frames++;
    }
  }

  async stop(): Promise<RecordingResult> { return this.exclusive(() => this.finish()); }

  private async finish(): Promise<RecordingResult> {
    const take = this.take;
    if (!take) throw new Error('No recording is active');
    clearInterval(take.timer);
    this.tick(take);
    const durationMs = Math.round(performance.now() - take.started);
    take.stopping = true;
    try {
      await bounded(take.session.send('Page.stopScreencast'), 'CDP recording stop');
      take.encoder.stdin.end();
      const exit = await bounded(take.exited, 'ffmpeg finalization');
      if (take.frames === 0) throw new Error('No frames captured');
      if (take.failure) throw take.failure;
      if (exit.code !== 0) throw new Error(`ffmpeg exited: ${exit.signal ?? exit.code}: ${ffmpegErrorTail(take.stderr)}`);
      const { size } = await stat(take.destination.temporary);
      if (size === 0) throw new Error('ffmpeg produced an empty recording');
      await rename(take.destination.temporary, take.destination.path);
      return { path: take.destination.path, frames: take.frames, capturedFrames: take.captured,
        fps: take.destination.fps, durationMs, bytes: size };
    } finally {
      take.encoder.stdin.destroy();
      if (take.encoder.exitCode === null && take.encoder.signalCode === null) take.encoder.kill('SIGKILL');
      await bounded(take.exited, 'ffmpeg cleanup').catch(() => {});
      await bounded(take.session.detach(), 'CDP recording detach').catch(() => {});
      await rm(take.destination.directory, { recursive: true, force: true });
      this.take = null;
    }
  }

  async restart(page: Page, path: string, fps?: number): Promise<{ previousPath: string | null; status: RecordingStatus }> {
    return this.exclusive(async () => {
      // Invalid paths, rates, missing encoders and unwritable parents leave the take running.
      const destination = await this.prepare(path, fps);
      let previousPath: string | null = null;
      try {
        if (this.take) previousPath = (await this.finish()).path;
        return { previousPath, status: await this.begin(page, destination) };
      } catch (error) {
        await rm(destination.directory, { recursive: true, force: true });
        throw error;
      }
    });
  }

  async shutdown(): Promise<void> {
    if (this.take) await this.stop().catch(() => {});
  }
}
