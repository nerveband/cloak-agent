import { describe, it, expect } from 'vitest';
import { processAriaTree, parseRef, getSnapshotStats, resetRefs } from '../src/snapshot.js';

describe('parseRef', () => {
  it('parses @e1', () => expect(parseRef('@e1')).toBe('e1'));
  it('parses ref=e5', () => expect(parseRef('ref=e5')).toBe('e5'));
  it('parses bare e3', () => expect(parseRef('e3')).toBe('e3'));
  it('rejects CSS selectors', () => expect(parseRef('#btn')).toBeNull());
  it('rejects random text', () => expect(parseRef('hello')).toBeNull());
});

describe('processAriaTree', () => {
  const tree = `- heading "Example" [level=1]\n- paragraph: Some text\n- button "Submit"\n- textbox "Email"\n- link "Click"`;

  it('interactive mode: only interactive elements get refs', () => {
    resetRefs();
    const refs = {};
    const result = processAriaTree(tree, refs, { interactive: true });
    expect(result).toContain('[ref=e1]');
    expect(result).toContain('button "Submit"');
    expect(result).toContain('textbox "Email"');
    expect(result).toContain('link "Click"');
    expect(result).not.toContain('paragraph');
    expect(Object.keys(refs)).toHaveLength(3);
  });

  it('full mode: interactive + named content elements get refs', () => {
    resetRefs();
    const refs = {};
    const result = processAriaTree(tree, refs, {});
    expect(result).toContain('[ref=');
    expect(result).toContain('heading');
    expect(result).toContain('paragraph');
    expect(Object.keys(refs).length).toBeGreaterThanOrEqual(4);
  });
});

describe('getSnapshotStats', () => {
  it('returns token estimate', () => {
    const stats = getSnapshotStats('hello world testdata', { e1: { role: 'button', selector: "getByRole('button')" } });
    expect(stats.tokens).toBe(5); // ceil(20/4)
    expect(stats.refs).toBe(1);
    expect(stats.chars).toBe(20);
    expect(stats.lines).toBe(1);
  });
});
