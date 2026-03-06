import { describe, it, expect } from 'vitest';
import { BrowserManager } from '../src/browser.js';

describe('BrowserManager', () => {
  it('starts not launched', () => {
    const bm = new BrowserManager();
    expect(bm.isLaunched()).toBe(false);
  });

  it('resolveRef returns null for unknown ref', () => {
    const bm = new BrowserManager();
    expect(bm.resolveRef('e99')).toBeNull();
  });

  it('getRefMap returns empty map initially', () => {
    const bm = new BrowserManager();
    expect(Object.keys(bm.getRefMap())).toHaveLength(0);
  });

  it('getPage throws when not launched', () => {
    const bm = new BrowserManager();
    expect(() => bm.getPage()).toThrow();
  });

  it('getConsoleMessages returns empty initially', () => {
    const bm = new BrowserManager();
    expect(bm.getConsoleMessages()).toEqual([]);
  });

  it('getPageErrors returns empty initially', () => {
    const bm = new BrowserManager();
    expect(bm.getPageErrors()).toEqual([]);
  });

  it('getTrackedRequests returns empty initially', () => {
    const bm = new BrowserManager();
    expect(bm.getTrackedRequests()).toEqual([]);
  });

  it('getTabList returns empty when not launched', () => {
    const bm = new BrowserManager();
    expect(bm.getTabList()).toEqual([]);
  });
});
