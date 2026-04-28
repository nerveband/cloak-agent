import * as os from 'node:os';
import * as path from 'node:path';
import * as fs from 'node:fs';

// ── Types ──────────────────────────────────────────────────────────────────────

export interface StealthOptions {
  fingerprintSeed?: number;
  platform?: 'windows' | 'macos' | 'linux';
  gpuVendor?: string;
  gpuRenderer?: string;
  args?: string[];
}

export interface StealthConfig {
  viewport: { width: number; height: number };
  platform: string;
}

// ── Constants ──────────────────────────────────────────────────────────────────

export const DATA_DIR = path.join(os.homedir(), '.cloak-agent');
export const PROFILES_DIR = path.join(DATA_DIR, 'profiles');

// ── Helpers ────────────────────────────────────────────────────────────────────

const isMac = os.platform() === 'darwin';

/** Extract the key portion of a CLI arg (everything before the first `=`). */
function argKey(arg: string): string {
  const idx = arg.indexOf('=');
  return idx === -1 ? arg : arg.slice(0, idx);
}

// ── Public API ─────────────────────────────────────────────────────────────────

/**
 * Returns sensible defaults for viewport size and platform.
 */
export function getDefaultStealthConfig(): StealthConfig {
  return {
    viewport: { width: 1920, height: 947 },
    platform: isMac ? 'macos' : 'windows',
  };
}

/**
 * Convert cloak-agent's legacy explicit fingerprint overrides to Chromium args.
 * CloakBrowser owns default stealth args; this only preserves compatibility for
 * callers who deliberately set seed/platform/GPU overrides.
 */
export function buildStealthArgs(options: StealthOptions): string[] {
  const args: string[] = [];

  if (options.fingerprintSeed !== undefined) {
    args.push(`--fingerprint=${options.fingerprintSeed}`);
  }
  if (options.platform) {
    args.push(`--fingerprint-platform=${options.platform}`);
  }
  if (options.gpuVendor) {
    args.push(`--fingerprint-gpu-vendor=${options.gpuVendor}`);
  }
  if (options.gpuRenderer) {
    args.push(`--fingerprint-gpu-renderer=${options.gpuRenderer}`);
  }

  // Merge user-supplied args with deduplication (by key before `=`).
  if (options.args && options.args.length > 0) {
    const existingKeys = new Set(args.map(argKey));
    for (const userArg of options.args) {
      const key = argKey(userArg);
      if (!existingKeys.has(key)) {
        args.push(userArg);
        existingKeys.add(key);
      }
    }
  }

  return args;
}

/**
 * Return the absolute path for a named browser profile.
 */
export function getProfileDir(name: string): string {
  return path.join(PROFILES_DIR, name);
}

/**
 * List all existing profile directory names. Returns an empty array when the
 * profiles directory does not yet exist.
 */
export function listProfiles(): string[] {
  try {
    const entries = fs.readdirSync(PROFILES_DIR, { withFileTypes: true });
    return entries.filter((e) => e.isDirectory()).map((e) => e.name);
  } catch {
    return [];
  }
}

/**
 * Ensure a profile directory exists and return its absolute path.
 */
export function ensureProfileDir(name: string): string {
  const dir = getProfileDir(name);
  fs.mkdirSync(dir, { recursive: true });
  return dir;
}
