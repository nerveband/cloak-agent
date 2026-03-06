import { describe, it, expect } from 'vitest';
import { buildStealthArgs, getProfileDir, listProfiles, getDefaultStealthConfig } from '../src/stealth.js';

describe('buildStealthArgs', () => {
  it('generates args with random fingerprint seed', () => {
    const args = buildStealthArgs({});
    expect(args.some(a => a.startsWith('--fingerprint='))).toBe(true);
    expect(args).toContain('--no-sandbox');
    expect(args).toContain('--disable-blink-features=AutomationControlled');
  });

  it('uses provided fingerprint seed', () => {
    const args = buildStealthArgs({ fingerprintSeed: 42 });
    expect(args).toContain('--fingerprint=42');
  });

  it('sets macos platform with Apple GPU', () => {
    const args = buildStealthArgs({ platform: 'macos' });
    expect(args).toContain('--fingerprint-platform=macos');
    expect(args.some(a => a.includes('Apple'))).toBe(true);
  });

  it('sets windows platform with NVIDIA GPU', () => {
    const args = buildStealthArgs({ platform: 'windows' });
    expect(args).toContain('--fingerprint-platform=windows');
    expect(args.some(a => a.includes('NVIDIA'))).toBe(true);
  });

  it('allows custom GPU', () => {
    const args = buildStealthArgs({ gpuVendor: 'AMD', gpuRenderer: 'RX 7900' });
    expect(args).toContain('--fingerprint-gpu-vendor=AMD');
    expect(args).toContain('--fingerprint-gpu-renderer=RX 7900');
  });

  it('deduplicates user args', () => {
    const args = buildStealthArgs({ args: ['--no-sandbox', '--custom=true'] });
    expect(args.filter(a => a === '--no-sandbox')).toHaveLength(1);
    expect(args).toContain('--custom=true');
  });

  it('sets timezone and locale', () => {
    const args = buildStealthArgs({ timezone: 'America/New_York', locale: 'en-US' });
    expect(args).toContain('--fingerprint-timezone=America/New_York');
    expect(args).toContain('--lang=en-US');
  });
});

describe('getDefaultStealthConfig', () => {
  it('returns viewport and platform', () => {
    const config = getDefaultStealthConfig();
    expect(config.viewport.width).toBe(1920);
    expect(config.viewport.height).toBe(947);
    expect(config).toHaveProperty('platform');
  });
});

describe('profiles', () => {
  it('getProfileDir returns path with .cloak-agent/profiles', () => {
    const dir = getProfileDir('test');
    expect(dir).toContain('.cloak-agent');
    expect(dir).toContain('profiles');
    expect(dir).toContain('test');
  });

  it('listProfiles returns array', () => {
    const profiles = listProfiles();
    expect(Array.isArray(profiles)).toBe(true);
  });
});
