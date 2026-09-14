import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { BrowserManager } from '../src/browser.js';
import { executeCommand } from '../src/actions.js';

// Native browser regressions, run in the browser integration job.
describe('session page safety', () => {
  let browser: BrowserManager;
  beforeEach(async () => {
    browser = new BrowserManager();
    await browser.launch({ webmcp: true, viewport: { width: 320, height: 240 } });
    await browser.getContext().route('https://review.invalid/**', route => route.fulfill({ contentType: 'text/html', body:
      '<button style="position:absolute;left:0;top:0;width:100px;height:60px" onclick="document.body.dataset.clicked=1">Same</button>' }));
    await browser.getPage().goto('https://review.invalid/');
  });
  afterEach(async () => { await browser.close(); });

  it('rejects refs on tab creation, switching, closing, and navigation', async () => {
    const first = browser.getPage();
    const snapshot = await browser.getSnapshot({ interactive: true });
    const ref = Object.keys(snapshot.refs)[0];
    await browser.newTab('https://review.invalid/second');
    expect(() => browser.getLocatorForRef(ref)).toThrow('Unknown ref');
    await browser.getSnapshot({ interactive: true });
    await browser.switchTab(0);
    expect(() => browser.getLocatorForRef(ref)).toThrow('Unknown ref');
    await browser.getSnapshot({ interactive: true });
    await first.goto('https://review.invalid/next');
    expect(() => browser.getLocatorForRef(ref)).toThrow('Unknown ref');
    await browser.getSnapshot({ interactive: true });
    await browser.closeTab(0);
    expect(() => browser.getLocatorForRef(ref)).toThrow('Unknown ref');
  });

  it('merges sequential media settings on new pages', async () => {
    await browser.setMediaEmulation({ colorScheme: 'dark' });
    await browser.setMediaEmulation({ media: 'print' });
    const page = await browser.newTab('https://review.invalid/media');
    expect(await page.evaluate(() => ({ dark: matchMedia('(prefers-color-scheme:dark)').matches, print: matchMedia('print').matches }))).toEqual({ dark: true, print: true });
  });

  it('sends streamed input to the pictured page even after active-tab changes', async () => {
    const first = browser.getPage();
    await browser.startScreencast(() => {});
    const second = await browser.newTab('https://review.invalid/second');
    await browser.injectMouseEvent({ action: 'click', x: 40, y: 30 });
    expect(await first.evaluate(() => document.body.dataset.clicked)).toBe('1');
    expect(await second.evaluate(() => document.body.dataset.clicked)).toBeUndefined();
    await browser.stopScreencast();
    await expect(browser.injectMouseEvent({ action: 'click', x: 40, y: 30 })).rejects.toThrow('No streamed page');
  });

  it('discovers and invokes real WebMCP tools on a secure frame', async () => {
    await browser.getPage().evaluate(`navigator.modelContext.registerTool({name:'echo',description:'Echo input',inputSchema:{type:'object',properties:{value:{type:'string'}}},execute:async({value})=>({content:[{type:'text',text:value}]})})`);
    const list = await executeCommand(browser, { id: 'list', action: 'webmcp_list' });
    expect(list.ok).toBe(true);
    if (!list.ok) throw new Error(list.error);
    expect(list.data).toMatchObject({ origin: 'https://review.invalid', tools: [{ name: 'echo' }] });
    const call = await executeCommand(browser, { id: 'call', action: 'webmcp_call', name: 'echo', arguments: { value: 'proof' } });
    expect(call.ok).toBe(true);
    if (!call.ok) throw new Error(call.error);
    expect(JSON.parse((call.data as { output: string }).output)).toEqual({ content: [{ type: 'text', text: 'proof' }] });
  });

  it('bounds non-resolving WebMCP tools and rejects unknown tools', async () => {
    await browser.getPage().evaluate(`navigator.modelContext.registerTool({name:'never',description:'Never resolves',inputSchema:{type:'object'},execute:()=>new Promise(()=>{})})`);
    const missing = await executeCommand(browser, { id: 'missing', action: 'webmcp_call', name: 'missing' });
    expect(missing.ok).toBe(false);
    const timed = await executeCommand(browser, { id: 'timeout', action: 'webmcp_call', name: 'never', timeout: 50 });
    expect(timed.ok).toBe(false);
    if (!timed.ok) expect(timed.error).toContain('timed out');
  });
});
