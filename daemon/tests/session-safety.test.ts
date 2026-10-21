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
  }, 60000);
  afterEach(async () => { await browser.close(); }, 60000);

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

  it('discovers and invokes real WebMCP tools on a secure frame', async (test) => {
    if (process.platform !== 'linux' && !await browser.getPage().evaluate(() => 'modelContext' in navigator)) {
      expect(await executeCommand(browser, { id: 'unsupported', action: 'webmcp_list' })).toMatchObject({ ok: false, error: expect.stringContaining('WebMCP unsupported') });
      test.skip('Installed browser does not expose WebMCP');
    }
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

  it('bounds non-resolving WebMCP tools and rejects unknown tools', async (test) => {
    if (process.platform !== 'linux' && !await browser.getPage().evaluate(() => 'modelContext' in navigator)) {
      expect(await executeCommand(browser, { id: 'unsupported', action: 'webmcp_list' })).toMatchObject({ ok: false, error: expect.stringContaining('WebMCP unsupported') });
      test.skip('Installed browser does not expose WebMCP');
    }
    await browser.getPage().evaluate(`navigator.modelContext.registerTool({name:'never',description:'Never resolves',inputSchema:{type:'object'},execute:()=>new Promise(()=>{})})`);
    const missing = await executeCommand(browser, { id: 'missing', action: 'webmcp_call', name: 'missing' });
    expect(missing.ok).toBe(false);
    const timed = await executeCommand(browser, { id: 'timeout', action: 'webmcp_call', name: 'never', timeout: 50 });
    expect(timed.ok).toBe(false);
    if (!timed.ok) expect(timed.error).toContain('timed out');
  });
  it('keeps refs on surviving nodes and never retargets a replaced element', async () => {
    const first = await browser.getSnapshot({ interactive: true });
    const ref = Object.keys(first.refs)[0];
    expect(await executeCommand(browser, { id: 'count-live', action: 'count', selector: `@${ref}` })).toMatchObject({ ok: true, data: { count: 1 } });
    await browser.getPage().evaluate(() => document.body.append(document.createElement('p')));
    expect(Object.keys((await browser.getSnapshot({ interactive: true })).refs)).toContain(ref);
    await browser.getPage().evaluate(() => {
      const button = document.querySelector('button')!;
      button.replaceWith(button.cloneNode(true));
    });
    expect(await executeCommand(browser, { id: 'count-detached', action: 'count', selector: `@${ref}` })).toMatchObject({ ok: true, data: { count: 0 } });
    const fresh = await browser.getSnapshot({ interactive: true });
    expect(Object.keys(fresh.refs)).not.toContain(ref);
  });

  it('scopes snapshots to the selected root including shadow content', async () => {
    await browser.getPage().evaluate(html => { document.body.innerHTML = html; }, '<button>Outside</button><main id="scope"><div><button>Inside</button></div><x-test></x-test></main>');
    await browser.getPage().evaluate(() => document.querySelector('x-test')!.attachShadow({ mode: 'open' }).innerHTML = '<button>Shadow</button>');
    const result = await browser.getSnapshot({ selector: '#scope', interactive: true });
    expect(result.tree).not.toContain('Outside');
    expect(result.tree.match(/Inside/g)).toHaveLength(1);
    expect(result.tree.match(/Shadow/g)).toHaveLength(1);
    const shadowRef = Object.entries(result.refs).find(([, data]) => data.name === 'Shadow')![0];
    expect(await browser.getLocatorForRef(shadowRef).count()).toBe(1);
    const root = await browser.getSnapshot({ selector: '#scope > div > button', interactive: true });
    expect(root.tree).toContain('Inside');
    expect(Object.keys(root.refs)).toHaveLength(1);
  });

  it('returns a delta baseline, unchanged revision, reconstructible patch, and forced full tree', async () => {
    const capture = async (extra = {}) => {
      const result = await executeCommand(browser, { id: 'delta', action: 'snapshot', interactive: true, delta: true, ...extra });
      if (!result.ok) throw new Error(result.error);
      return result.data as any;
    };
    const baseline = await capture();
    expect(baseline.baseline).toBe(true);
    const unchanged = await capture();
    expect(unchanged.unchanged).toBe(true);
    expect(unchanged.revision).toBe(baseline.revision);
    expect(unchanged.tree).toBeUndefined();
    await browser.getPage().evaluate(() => document.querySelector('button')!.textContent = 'Changed');
    const changed = await capture();
    const lines = baseline.tree.split('\n');
    lines.splice(changed.delta.start, changed.delta.deleteCount, ...changed.delta.lines);
    expect(lines.join('\n')).toBe((await browser.getSnapshot({ interactive: true })).tree);
    expect((await capture({ full: true })).baseline).toBe(true);
    await browser.getPage().goto('https://review.invalid/next');
    expect((await capture()).baseline).toBe(true);
  });

  it('skips unchanged screenshots without writing a file and isolates capture modes and tabs', async () => {
    const capture = async (extra = {}) => {
      const result = await executeCommand(browser, { id: 'image', action: 'screenshot', ifChanged: true, ...extra });
      if (!result.ok) throw new Error(result.error);
      return result.data as any;
    };
    expect((await capture()).changed).toBe(true);
    const same = await capture({ path: '/missing-directory/should-not-be-written.png' });
    expect(same).toEqual({ changed: false, difference: 0 });
    expect((await capture({ fullPage: true })).changed).toBe(true);
    await browser.getPage().evaluate(() => document.body.style.background = 'red');
    expect((await capture({ threshold: 1 })).changed).toBe(false);
    expect((await capture()).changed).toBe(true);
    await browser.newTab('https://review.invalid/second');
    expect((await capture()).changed).toBe(true);
  });

});
