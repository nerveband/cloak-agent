import { execFileSync, spawnSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { describe, expect, it, vi } from 'vitest';
import type { CDPSession, Page } from 'playwright-core';

import {
  DEFAULT_FPS,
  RecordingManager,
  buildFfmpegArgs,
  checkFfmpegAvailable,
  ffmpegErrorTail,
  framesDue,
  outputExtension,
  validateRecordingFps,
  validateRecordingPath,
} from '../src/recording.js';

const ffmpegAvailable = spawnSync('ffmpeg', ['-version'], { stdio: 'ignore' }).status === 0;

/** A 64x48 JPEG frame the encoder can actually decode. */
function sampleJpeg(): Buffer {
  return execFileSync('ffmpeg', [
    '-hide_banner',
    '-loglevel', 'error',
    '-f', 'lavfi',
    '-i', 'color=c=blue:s=64x48',
    '-frames:v', '1',
    '-f', 'mjpeg',
    '-',
  ]);
}

interface FakeCapture {
  page: Page;
  emitFrame(): void;
}

/** Test double: only the CDP session factory and screencast events are used. */
function fakePage(): FakeCapture {
  const handlers = new Map<string, (params: unknown) => void>();
  const session = {
    on: (event: string, handler: (params: unknown) => void) => {
      handlers.set(event, handler);
    },
    send: async () => ({}),
    detach: async () => {},
  };
  const page = {
    context: () => ({ newCDPSession: async () => session as unknown as CDPSession }),
  } as unknown as Page;

  return {
    page,
    emitFrame: () => {
      const handler = handlers.get('Page.screencastFrame');
      if (!handler) {
        throw new Error('Screencast handler was not registered.');
      }
      handler({ data: sampleJpeg().toString('base64'), sessionId: 1 });
    },
  };
}

describe('recording helpers', () => {
  it('reads the container extension', () => {
    expect(outputExtension('demo.webm')).toBe('webm');
    expect(outputExtension('demo.MP4')).toBe('mp4');
    expect(outputExtension('demo')).toBeNull();
    expect(outputExtension('/tmp/take.one/two')).toBeNull();
  });

  it('rejects paths without an extension', () => {
    expect(() => validateRecordingPath('take')).toThrow(/has no extension/);
    expect(() => validateRecordingPath('take.webm')).not.toThrow();
  });

  it('rejects out-of-range frame rates', () => {
    expect(validateRecordingFps(30)).toBe(30);
    expect(() => validateRecordingFps(0)).toThrow(/out of range/);
    expect(() => validateRecordingFps(61)).toThrow(/out of range/);
    expect(() => validateRecordingFps(29.5)).toThrow(/out of range/);
  });

  it('builds a libvpx command for webm and libx264 for other containers', () => {
    const webm = buildFfmpegArgs('/tmp/take.webm', 30);
    expect(webm).toContain('libvpx');
    expect(webm).toContain('1000k');
    expect(webm[webm.indexOf('-framerate') + 1]).toBe('30');
    expect(webm[webm.length - 1]).toBe('/tmp/take.webm');

    const fastWebm = buildFfmpegArgs('/tmp/take.webm', 60);
    expect(fastWebm).toContain('2000k');
    expect(fastWebm[fastWebm.indexOf('-threads') + 1]).toBe('2');

    const mp4 = buildFfmpegArgs('/tmp/take.mp4', 30);
    expect(mp4).toContain('libx264');
    expect(mp4).not.toContain('libvpx');
  });

  it('keeps the tail of ffmpeg stderr on a line boundary', () => {
    expect(ffmpegErrorTail('short failure')).toBe('short failure');

    const lines = ['configure line', ...Array.from({ length: 80 }, (_, index) => `banner line ${index}`), 'Error while opening encoder'];
    const tail = ffmpegErrorTail(lines.join('\n'));
    expect(tail.length).toBeLessThanOrEqual(400);
    expect(tail).toContain('Error while opening encoder');
    expect(tail).not.toContain('configure line');
    // The window starts on a whole line, never mid-token.
    expect(/^banner line \d+/.test(tail)).toBe(true);
  });

  it('computes frames owed to the constant-rate stream', () => {
    expect(framesDue(0, 33, 0)).toBe(1);
    expect(framesDue(1000, 33, 0)).toBe(31);
    expect(framesDue(1000, 33, 31)).toBe(0);
    expect(framesDue(10, 33, 5)).toBe(0);
  });
});

describe('RecordingManager', () => {
  it.skipIf(!ffmpegAvailable)('records screencast frames into a playable file', async () => {
    vi.useFakeTimers();
    try {
      const capture = fakePage();
      const manager = new RecordingManager();
      const outPath = path.join(fs.mkdtempSync(path.join(os.tmpdir(), 'cloak-rec-')), 'take.webm');

      const status = await manager.start(capture.page, outPath);
      expect(status).toMatchObject({ active: true, path: outPath, fps: DEFAULT_FPS, capturedFrames: 0 });

      for (let i = 0; i < 10; i += 1) {
        capture.emitFrame();
      }
      expect(manager.status().capturedFrames).toBe(10);

      // Advance the recorder clock instead of waiting on it: 12 slots at 30 fps.
      vi.advanceTimersByTime(400);
      expect(manager.status().frames).toBeGreaterThan(0);

      const result = await manager.stop();

      expect(result.path).toBe(outPath);
      expect(result.fps).toBe(DEFAULT_FPS);
      expect(result.frames).toBeGreaterThan(0);
      expect(result.bytes).toBeGreaterThan(0);
      expect(fs.statSync(outPath).size).toBeGreaterThan(0);
      expect(manager.status().active).toBe(false);
    } finally {
      vi.useRealTimers();
    }
  });

  it.skipIf(!ffmpegAvailable)('reports an empty take instead of leaving a broken file', async () => {
    const capture = fakePage();
    const manager = new RecordingManager();
    const outPath = path.join(fs.mkdtempSync(path.join(os.tmpdir(), 'cloak-rec-')), 'empty.webm');

    await manager.start(capture.page, outPath);
    await expect(manager.stop()).rejects.toThrow(/No frames captured/);
    expect(manager.status().active).toBe(false);
  });

  it('rejects a second take while one is active', async () => {
    const capture = fakePage();
    const manager = new RecordingManager();

    await manager.start(capture.page, path.join(os.tmpdir(), 'cloak-rec-first.webm'), 15);
    await expect(
      manager.start(capture.page, path.join(os.tmpdir(), 'cloak-rec-second.webm')),
    ).rejects.toThrow(/already active/);
    await manager.shutdown();
  });

  it('fails fast with the ffmpeg install hint when the binary is missing', async () => {
    const originalPath = process.env.PATH;
    process.env.PATH = '/nonexistent-cloak-agent-path';
    try {
      await expect(checkFfmpegAvailable()).rejects.toThrow(/Install ffmpeg/);

      const manager = new RecordingManager();
      await expect(manager.start(fakePage().page, '/tmp/missing-ffmpeg.webm')).rejects.toThrow(/Install ffmpeg/);
      expect(manager.status().active).toBe(false);
    } finally {
      process.env.PATH = originalPath;
    }
  });

  it('validates before touching the browser', async () => {
    const capture = fakePage();
    const manager = new RecordingManager();
    await expect(manager.start(capture.page, '/tmp/no-extension')).rejects.toThrow(/has no extension/);
    await expect(manager.start(capture.page, '/tmp/ok.webm', 90)).rejects.toThrow(/out of range/);
    expect(manager.status().active).toBe(false);
  });
});
