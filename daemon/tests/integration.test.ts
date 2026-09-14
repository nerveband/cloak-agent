import { describe, it, expect, afterAll } from 'vitest';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { BrowserManager } from '../src/browser.js';
import { getSnapshotStats } from '../src/snapshot.js';

describe('integration: stealth browser', { timeout: 120000 }, () => {
  const browser = new BrowserManager();

  afterAll(async () => {
    await browser.close();
  }, 30000);

  it('launches CloakBrowser and navigates', async () => {
    await browser.launch({ headless: true });
    expect(browser.isLaunched()).toBe(true);

    const page = browser.getPage();
    await page.goto('https://example.com');
    const title = await page.title();
    expect(title).toContain('Example');
  }, 60000);

  it('takes interactive snapshot with refs', async () => {
    const result = await browser.getSnapshot({ interactive: true });
    expect(result.tree).toContain('[ref=');
    expect(Object.keys(result.refs).length).toBeGreaterThan(0);
  }, 30000);

  it('snapshot includes token estimate in stats', async () => {
    const result = await browser.getSnapshot({ interactive: true });
    const stats = getSnapshotStats(result.tree, result.refs);
    expect(stats.tokens).toBeGreaterThan(0);
    expect(stats.refs).toBeGreaterThan(0);
  }, 30000);

  it('can get page title and url', async () => {
    const page = browser.getPage();
    const title = await page.title();
    const url = page.url();
    expect(title).toContain('Example');
    expect(url).toContain('example.com');
  });

  it('tab list shows one tab', () => {
    const tabs = browser.getTabList();
    expect(tabs.length).toBe(1);
    expect(tabs[0].url).toContain('example.com');
  });

  it('reports runtime status with active page proof', async () => {
    const status = await browser.getRuntimeStatus();
    expect(status.launched).toBe(true);
    expect(status.activePageIndex).toBe(0);
    expect(status.activePage?.title).toContain('Example');
    expect(status.tabs.length).toBe(1);
  });

  it('sends raw CDP commands to the active page', async () => {
    const result: unknown = await browser.sendCDP('Runtime.evaluate', {
      expression: 'document.title',
      returnByValue: true,
    });
    expect(result).toMatchObject({ result: { value: expect.stringContaining('Example') } });
  });

  it('invalidates refs when the page navigates', async () => {
    const snapshot = await browser.getSnapshot({ interactive: true });
    const firstRef = Object.keys(snapshot.refs)[0];
    expect(firstRef).toBeDefined();
    expect(browser.resolveRef(firstRef)).not.toBeNull();

    await browser.getPage().goto('https://example.com/?refresh=1');
    expect(browser.resolveRef(firstRef)).toBeNull();
  });

  it('inherits page-level emulation in new tabs', async () => {
    await browser.setMediaEmulation({ colorScheme: 'dark' });
    const before = browser.getTabList().length;
    const newPage = await browser.newTab('https://example.com');
    expect(browser.getTabList().length).toBe(before + 1);

    const prefersDark = await newPage.evaluate(() => matchMedia('(prefers-color-scheme: dark)').matches);
    expect(prefersDark).toBe(true);

    const newTabId = browser.getTabList().at(-1)?.tabId;
    if (!newTabId) {
      throw new Error('Expected the new tab to be listed.');
    }
    await browser.closeTabTarget(newTabId);
    await browser.setMediaEmulation({ colorScheme: null });
  });

  it('records the active page to a webm file', async () => {
    const outDir = fs.mkdtempSync(path.join(os.tmpdir(), 'cloak-agent-rec-'));
    const outPath = path.join(outDir, 'take.webm');

    const started = await browser.startRecording(outPath, 15);
    expect(started).toMatchObject({ active: true, fps: 15 });

    // Real wall-clock capture: CDP pushes screencast frames on repaint, so the
    // recorder needs genuine elapsed time that fake timers cannot supply.
    const { promise, resolve } = Promise.withResolvers<void>();
    setTimeout(resolve, 1500);
    await promise;

    const result = await browser.stopRecording();
    expect(result.path).toBe(outPath);
    expect(result.frames).toBeGreaterThan(0);
    expect(fs.statSync(outPath).size).toBeGreaterThan(0);
    expect(browser.recordingStatus().active).toBe(false);
  }, 60000);
});
