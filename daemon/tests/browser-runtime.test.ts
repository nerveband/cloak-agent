import { mkdtempSync, readFileSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { describe, expect, it } from 'vitest';
import { assertPatchedChromium, fingerprintSeedArg, parseChromiumVersion } from '../src/browser-runtime.js';

describe('Chromium version floor', () => {
  it('rejects builds older than 156.0.8078.73', () => {
    for (const version of ['145.0.7632.109', '155.0.8059.39', '156.0.8078.47', '156.0.8078.72']) {
      expect(() => assertPatchedChromium('/bin/chrome', version), version).toThrow(/CVE-2026-118306/);
    }
  });

  it('accepts the fixed build and newer, comparing numerically', () => {
    for (const version of ['156.0.8078.73', '156.0.8078.80', '156.0.8100.0', '157.0.1.0', '1000.0.0.0']) {
      expect(assertPatchedChromium('/bin/chrome', version)).toBe(version);
    }
  });

  it('refuses a binary whose version cannot be read', () => {
    expect(() => assertPatchedChromium('/bin/chrome', null)).toThrow(/unverified/);
  });

  it('reads Chrome and CloakBrowser --version output', () => {
    expect(parseChromiumVersion('Google Chrome 156.0.8078.74 \n')).toBe('156.0.8078.74');
    expect(parseChromiumVersion('Chromium 156.0.8078.47.1')).toBe('156.0.8078.47');
  });
});

describe('profile fingerprint seed', () => {
  it('keeps one seed per profile and replaces a corrupt seed file', () => {
    const dir = mkdtempSync(join(tmpdir(), 'cloak-seed-'));
    const first = fingerprintSeedArg(dir);
    expect(fingerprintSeedArg(dir)).toBe(first);
    expect(readFileSync(join(dir, '.cloakbrowser-seed'), 'latin1').trim()).toBe(first.split('=')[1]);
    writeFileSync(join(dir, '.cloakbrowser-seed'), 'garbage');
    expect(fingerprintSeedArg(dir)).toMatch(/^--fingerprint=\d{5}$/);
  });
});
