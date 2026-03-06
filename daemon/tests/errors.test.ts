import { describe, it, expect } from 'vitest';
import { toAIFriendlyError, validateFilePath, sanitizeInput, validateRef } from '../src/errors.js';

describe('toAIFriendlyError', () => {
  it('strict mode violation', () => {
    const err = new Error('strict mode violation: resolved to 3 elements');
    expect(toAIFriendlyError(err, '@e1').message).toContain('matched 3 elements');
  });
  it('pointer intercept', () => {
    const err = new Error('Element intercepts pointer events');
    expect(toAIFriendlyError(err, '@e1').message).toContain('blocked');
  });
  it('timeout', () => {
    const err = new Error('Timeout 30000ms exceeded');
    expect(toAIFriendlyError(err, '@e1').message).toContain('timed out');
  });
  it('not visible', () => {
    const err = new Error('Element is not visible');
    expect(toAIFriendlyError(err, '@e1').message).toContain('not visible');
  });
  it('passthrough', () => {
    const err = new Error('something else');
    expect(toAIFriendlyError(err, '@e1').message).toBe('something else');
  });
});

describe('validateFilePath', () => {
  it('rejects path traversal', () => {
    expect(() => validateFilePath('../../.ssh/id_rsa')).toThrow('path traversal');
  });
  it('rejects control characters', () => {
    expect(() => validateFilePath('file\x00name.png')).toThrow('control character');
  });
  it('accepts valid path', () => {
    expect(validateFilePath('/tmp/screenshot.png')).toBe('/tmp/screenshot.png');
  });
  it('accepts relative path without traversal', () => {
    const result = validateFilePath('output.png');
    expect(result).toContain('output.png');
  });
});

describe('sanitizeInput', () => {
  it('strips control chars but keeps newlines and tabs', () => {
    expect(sanitizeInput('hello\x00world\n\ttab')).toBe('helloworld\n\ttab');
  });
  it('passes through normal text', () => {
    expect(sanitizeInput('hello world')).toBe('hello world');
  });
});

describe('validateRef', () => {
  it('accepts valid ref', () => expect(validateRef('e1')).toBe(true));
  it('accepts large ref', () => expect(validateRef('e999')).toBe(true));
  it('rejects no number', () => expect(validateRef('e')).toBe(false));
  it('rejects CSS', () => expect(validateRef('#btn')).toBe(false));
  it('rejects empty', () => expect(validateRef('')).toBe(false));
});
