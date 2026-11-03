// Browser runtime selection: an installed, patched Chromium with its sandbox on.
//
// cloak-agent never downloads a browser. It launches the installed Google Chrome,
// or the binary named by --executable-path / CLOAK_AGENT_EXECUTABLE_PATH (for
// example a CloakBrowser build), and refuses any binary older than the Chromium
// release that fixed CVE-2026-118306 (sandbox escape) and its companions.

import { execFileSync } from 'node:child_process';
import * as fs from 'node:fs';
import * as path from 'node:path';

/** First Chromium release with the fixes for CVE-2026-118306/-118331/-118344/-118348. */
export const MIN_CHROMIUM_VERSION = '156.0.8078.73';

const SEED_FILE = '.cloakbrowser-seed';
const SEED_MIN = 10000;
const SEED_MAX = 99999;

/** Standard Google Chrome (stable) locations, matching Playwright's `chrome` channel. */
export function systemChromePaths(
  platform: NodeJS.Platform = process.platform,
  env: NodeJS.ProcessEnv = process.env,
): string[] {
  switch (platform) {
    case 'darwin':
      return [
        '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',
        ...(env.HOME ? [path.join(env.HOME, 'Applications/Google Chrome.app/Contents/MacOS/Google Chrome')] : []),
      ];
    case 'win32':
      return [env.LOCALAPPDATA, env.PROGRAMFILES, env['PROGRAMFILES(X86)']]
        .filter((dir): dir is string => Boolean(dir))
        .map(dir => path.win32.join(dir, 'Google', 'Chrome', 'Application', 'chrome.exe'));
    default:
      return ['/opt/google/chrome/chrome'];
  }
}

/** The explicit executable if given, else the first installed Google Chrome. */
export function resolveBrowserExecutable(explicit?: string): string {
  if (explicit) {
    if (!fs.existsSync(explicit)) {
      throw new Error(`Browser executable not found: ${explicit}`);
    }
    return explicit;
  }
  const found = systemChromePaths().find(candidate => fs.existsSync(candidate));
  if (!found) {
    throw new Error(
      'Google Chrome is not installed. Install Google Chrome, or pass --executable-path ' +
      `(CLOAK_AGENT_EXECUTABLE_PATH) to a Chromium build >= ${MIN_CHROMIUM_VERSION}.`,
    );
  }
  return found;
}

/** Parse the four-part Chromium version from `--version` output or a directory name. */
export function parseChromiumVersion(text: string): string | null {
  const match = /(\d+)\.(\d+)\.(\d+)\.(\d+)/.exec(text);
  return match ? match.slice(1, 5).join('.') : null;
}

/** Negative when a < b, zero when equal, positive when a > b (first four parts). */
export function compareChromiumVersions(a: string, b: string): number {
  const left = a.split('.').map(Number);
  const right = b.split('.').map(Number);
  for (let i = 0; i < 4; i++) {
    const diff = (left[i] ?? 0) - (right[i] ?? 0);
    if (diff !== 0) return diff;
  }
  return 0;
}

/** Read the Chromium version of a browser executable. */
export function readChromiumVersion(executable: string): string | null {
  if (process.platform === 'win32') {
    // chrome.exe --version prints nothing on Windows; its sibling directory is the version.
    const entries = fs.readdirSync(path.dirname(executable));
    const versions = entries.map(parseChromiumVersion).filter((v): v is string => v !== null);
    return versions.sort(compareChromiumVersions).at(-1) ?? null;
  }
  const output = execFileSync(executable, ['--version'], { encoding: 'utf8', timeout: 15_000 });
  return parseChromiumVersion(output);
}

/** Throw unless the executable reports a Chromium version at or above the floor. */
export function assertPatchedChromium(executable: string, version = readChromiumVersion(executable)): string {
  if (!version) {
    throw new Error(`Could not read the Chromium version of ${executable}; refusing to launch an unverified browser.`);
  }
  if (compareChromiumVersions(version, MIN_CHROMIUM_VERSION) < 0) {
    throw new Error(
      `${executable} is Chromium ${version}, older than ${MIN_CHROMIUM_VERSION} ` +
      '(CVE-2026-118306 sandbox escape). Update the browser or point --executable-path at a patched build.',
    );
  }
  return version;
}

/**
 * CloakBrowser's default stealth args include --no-sandbox, so cloak-agent turns
 * them off and supplies the fingerprint seed itself. A persistent profile keeps
 * its seed in `<userDataDir>/.cloakbrowser-seed`, the file CloakBrowser uses.
 */
export function fingerprintSeedArg(userDataDir?: string): string {
  const random = () => SEED_MIN + Math.floor(Math.random() * (SEED_MAX - SEED_MIN + 1));
  if (!userDataDir) return `--fingerprint=${random()}`;
  const file = path.join(userDataDir, SEED_FILE);
  try {
    const text = fs.readFileSync(file, 'latin1').trim();
    const seed = Number(text);
    if (/^[0-9]+$/.test(text) && seed >= SEED_MIN && seed <= SEED_MAX) return `--fingerprint=${seed}`;
  } catch (error) {
    if ((error as NodeJS.ErrnoException).code !== 'ENOENT') throw error;
  }
  const seed = random();
  fs.writeFileSync(file, `${seed}\n`);
  return `--fingerprint=${seed}`;
}
