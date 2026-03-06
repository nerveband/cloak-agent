import { describe, it, expect, afterAll } from 'vitest';
import { BrowserManager } from '../src/browser.js';
import { getSnapshotStats } from '../src/snapshot.js';

describe('integration: stealth browser', () => {
  const browser = new BrowserManager();

  afterAll(async () => {
    await browser.close();
  });

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
}, { timeout: 120000 });
