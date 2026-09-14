import { execFileSync, spawnSync } from 'node:child_process';
import type { ChildProcess } from 'node:child_process';
import { mkdtemp, readFile, readdir, rm, stat, writeFile } from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import { afterEach, describe, expect, it } from 'vitest';
import type { CDPSession, Page } from 'playwright-core';
import { RecordingManager, framesDue, validateRecordingFps } from '../src/recording.js';

const available = spawnSync('ffmpeg', ['-version'], { stdio: 'ignore' }).status === 0;
const directories: string[] = [];
const managers: RecordingManager[] = [];
const sleep = (ms: number) => new Promise(resolve => setTimeout(resolve, ms));
afterEach(async () => {
  for (const manager of managers.splice(0)) await manager.shutdown();
  for (const directory of directories.splice(0)) await rm(directory, { recursive: true, force: true });
});
async function fixture() {
  const directory = await mkdtemp(path.join(os.tmpdir(), 'cloak-rec-test-'));
  directories.push(directory);
  const handlers = new Map<string, (params: unknown) => void>();
  const session = {
    on(event: string, handler: (params: unknown) => void) { handlers.set(event, handler); },
    async send() { return {}; }, async detach() {},
  };
  const page = { context: () => ({ newCDPSession: async () => session as unknown as CDPSession }) } as unknown as Page;
  const jpeg = execFileSync('ffmpeg', ['-v', 'error', '-f', 'lavfi', '-i', 'color=c=blue:s=64x48', '-frames:v', '1', '-f', 'mjpeg', '-']);
  const manager = new RecordingManager(); managers.push(manager);
  return { directory, page, manager, emit: () => handlers.get('Page.screencastFrame')!({ data: jpeg.toString('base64'), sessionId: 1 }) };
}

describe('recording rate contract', () => {
  it('keeps exact frame counts across long timelines and non-divisor rates', () => {
    for (const fps of [1, 24, 30, 59, 60]) {
      expect(framesDue(3600000, fps, 0)).toBe(3600 * fps);
      expect(framesDue(3600000, fps, 3600 * fps)).toBe(0);
    }
  });
  it('rejects rates outside the contract', () => {
    for (const fps of [0, -1, 61, 29.5, NaN]) expect(() => validateRecordingFps(fps)).toThrow();
  });
});

describe.skipIf(!available)('recording output transaction', () => {
  it('writes decodable video and reports file bytes, including held static frames', async () => {
    const f = await fixture(); const output = path.join(f.directory, 'take.webm');
    await f.manager.start(f.page, output, 30); f.emit(); await sleep(350);
    const result = await f.manager.stop();
    expect(result.bytes).toBe((await stat(output)).size);
    const probe = JSON.parse(execFileSync('ffprobe', ['-v', 'error', '-show_entries', 'format=duration', '-of', 'json', output], { encoding: 'utf8' }));
    expect(Math.abs(Number(probe.format.duration) * 1000 - result.durationMs)).toBeLessThan(100);
    execFileSync('ffmpeg', ['-v', 'error', '-i', output, '-f', 'null', '-']);
    expect(await readdir(f.directory)).toEqual(['take.webm']);
  });
  it('keeps the current take active when replacement validation fails', async () => {
    const f = await fixture(); const output = path.join(f.directory, 'first.webm');
    await f.manager.start(f.page, output, 15); f.emit();
    for (const [file, fps] of [['no-extension', 15], ['invalid.webm', 0], ['invalid.unknownformat', 15], ['missing/next.webm', 15]] as const) {
      await expect(f.manager.restart(f.page, path.join(f.directory, file), fps)).rejects.toThrow();
      expect(f.manager.status().active).toBe(true);
    }
    await expect(f.manager.restart(f.page, output)).rejects.toThrow();
    await sleep(150); await f.manager.stop();
    expect(await readdir(f.directory)).toEqual(['first.webm']);
  });
  it('does not replace an existing file with an empty take', async () => {
    const f = await fixture(); const output = path.join(f.directory, 'existing.mp4');
    await writeFile(output, 'original');
    await f.manager.start(f.page, output);
    await expect(f.manager.stop()).rejects.toThrow('No frames captured');
    expect(await readFile(output, 'utf8')).toBe('original');
    expect(await readdir(f.directory)).toEqual(['existing.mp4']);
  });
  it('rejects a killed encoder instead of publishing success', async () => {
    const f = await fixture(); const output = path.join(f.directory, 'killed.webm');
    await writeFile(output, 'original');
    await f.manager.start(f.page, output); f.emit(); await sleep(200);
    // Fault injection kills only this manager's child, not an unrelated process.
    const encoder = (f.manager as unknown as { take: { encoder: ChildProcess } }).take.encoder;
    const exited = new Promise(resolve => encoder.once('close', resolve));
    encoder.kill('SIGKILL'); await exited;
    await expect(f.manager.stop()).rejects.toThrow();
    expect(await readFile(output, 'utf8')).toBe('original');
    expect(f.manager.status().active).toBe(false);
  });
});
