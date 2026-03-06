# Cloak Agent Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build `cloak-agent` — a CLI browser automation tool for AI agents that combines agent-browser's token-efficient snapshot/ref architecture with CloakBrowser's undetectable stealth Chromium binary.

**Architecture:** Unix-socket daemon process manages a stealth Chromium instance launched via CloakBrowser's `launch()`. A Rust-compiled native CLI binary parses commands, sends JSON over the socket, and prints compact responses. The snapshot engine uses Playwright's ARIA accessibility tree with `@ref` IDs so AI agents interact with ~50 lines instead of raw HTML. CloakBrowser's fingerprint randomization, GPU spoofing, and geoip proxy detection are first-class features exposed as CLI commands.

**Tech Stack:** TypeScript (ESM), Playwright-core, CloakBrowser npm package, Zod (validation), ws (WebSocket streaming), vitest (testing). CLI compiled to native binary via `pkg` or distributed as Node.js ESM entrypoint.

---

## Architecture Overview

```
┌──────────────┐     Unix Socket / TCP     ┌──────────────────┐
│  cloak-agent │  ───── JSON-RPC ────────> │   Daemon Process  │
│   CLI (bin)  │  <──── JSON response ──── │                   │
└──────────────┘                           │  BrowserManager   │
                                           │  ├─ CloakBrowser  │
                                           │  │  launch()       │
                                           │  ├─ SnapshotEngine│
                                           │  ├─ ActionExecutor│
                                           │  └─ StreamServer  │
                                           └──────────────────┘
```

**What we take from agent-browser (proven patterns):**
- Daemon architecture (Unix socket + TCP fallback)
- Snapshot engine (ARIA tree → `@ref` IDs → compact output)
- Protocol (JSON over socket, Zod validation)
- CLI command structure (verb-noun pattern)
- Session isolation (named sessions with separate sockets)
- AI-friendly error messages

**What we add from CloakBrowser (stealth layer):**
- Source-level patched Chromium binary (not runtime JS patches)
- Fingerprint seed randomization per launch
- GPU vendor/renderer spoofing (platform-aware: Apple GPU on macOS, NVIDIA on Linux/Windows)
- Proxy-aware GeoIP auto-detection (timezone + locale from proxy IP)
- Persistent profile support (avoids incognito detection penalties)
- `--fingerprint-*` CLI flags for full control

**What we improve over both:**
- `stealth status` command — run detection tests and report pass/fail
- `fingerprint rotate` command — regenerate fingerprint seed mid-session
- `profile create/load` — named persistent browser profiles with stealth defaults
- Simpler install — single `npm install -g cloak-agent` with auto binary download
- Token budget awareness — snapshot output includes token count estimate

---

## File Structure

```
cloak-agent/
├── package.json
├── tsconfig.json
├── vitest.config.ts
├── bin/
│   └── cloak-agent.js          # CLI entrypoint (ESM)
├── src/
│   ├── cli/
│   │   └── parser.ts           # CLI arg → JSON command parser
│   ├── lib/
│   │   ├── daemon.ts           # Socket server, session management
│   │   ├── browser.ts          # BrowserManager (CloakBrowser launch + Playwright)
│   │   ├── snapshot.ts         # ARIA snapshot engine with @refs
│   │   ├── actions.ts          # Command executor (all browser actions)
│   │   ├── protocol.ts         # Zod schemas, parse/serialize
│   │   ├── stealth.ts          # Stealth-specific: fingerprint, detection check
│   │   ├── stream-server.ts    # WebSocket viewport streaming
│   │   └── errors.ts           # AI-friendly error messages
│   └── index.ts                # Public API for programmatic use
├── tests/
│   ├── snapshot.test.ts
│   ├── protocol.test.ts
│   ├── stealth.test.ts
│   ├── cli-parser.test.ts
│   ├── browser.test.ts
│   └── integration.test.ts
└── docs/
    └── plans/
        └── 2026-03-06-cloak-agent-design.md
```

---

## Task 1: Project Scaffolding

**Files:**
- Create: `package.json`
- Create: `tsconfig.json`
- Create: `vitest.config.ts`
- Create: `.gitignore`

**Step 1: Create package.json**

```json
{
  "name": "cloak-agent",
  "version": "0.1.0",
  "description": "Stealth browser automation CLI for AI agents — undetectable Chromium with token-efficient snapshots",
  "type": "module",
  "main": "dist/index.js",
  "types": "dist/index.d.ts",
  "bin": {
    "cloak-agent": "./bin/cloak-agent.js"
  },
  "exports": {
    ".": {
      "types": "./dist/index.d.ts",
      "import": "./dist/index.js"
    }
  },
  "scripts": {
    "build": "tsc",
    "test": "vitest run",
    "test:watch": "vitest",
    "dev": "tsc --watch"
  },
  "engines": {
    "node": ">=18"
  },
  "dependencies": {
    "cloakbrowser": "^0.3.9",
    "playwright-core": "^1.57.0",
    "ws": "^8.19.0",
    "zod": "^3.22.4"
  },
  "devDependencies": {
    "@types/node": "^22.0.0",
    "@types/ws": "^8.5.10",
    "typescript": "^5.7.0",
    "vitest": "^3.0.0"
  },
  "license": "MIT"
}
```

**Step 2: Create tsconfig.json**

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "module": "NodeNext",
    "moduleResolution": "NodeNext",
    "outDir": "dist",
    "rootDir": "src",
    "declaration": true,
    "declarationMap": true,
    "sourceMap": true,
    "strict": true,
    "esModuleInterop": true,
    "skipLibCheck": true,
    "forceConsistentCasingInFileNames": true
  },
  "include": ["src/**/*"],
  "exclude": ["node_modules", "dist", "tests"]
}
```

**Step 3: Create vitest.config.ts**

```ts
import { defineConfig } from 'vitest/config';

export default defineConfig({
  test: {
    globals: true,
    testTimeout: 30000,
  },
});
```

**Step 4: Create .gitignore**

```
node_modules/
dist/
*.tsbuildinfo
.cloak-agent/
```

**Step 5: Install dependencies**

Run: `cd /Users/ashrafali/cloak-agent && npm install`
Expected: `added N packages`

**Step 6: Verify TypeScript compiles (empty project)**

Run: `mkdir -p src && echo 'export {};' > src/index.ts && npx tsc --noEmit`
Expected: No errors

**Step 7: Commit**

```bash
git add -A
git commit -m "feat: project scaffolding with TypeScript, vitest, dependencies"
```

---

## Task 2: Protocol Layer (Zod Schemas + JSON Serialization)

**Files:**
- Create: `src/lib/protocol.ts`
- Create: `tests/protocol.test.ts`

**Step 1: Write failing tests**

```ts
// tests/protocol.test.ts
import { describe, it, expect } from 'vitest';
import { parseCommand, successResponse, errorResponse, serializeResponse } from '../src/lib/protocol.js';

describe('parseCommand', () => {
  it('parses a valid navigate command', () => {
    const input = JSON.stringify({ id: '1', action: 'navigate', url: 'https://example.com' });
    const result = parseCommand(input);
    expect(result.success).toBe(true);
    if (result.success) {
      expect(result.command.action).toBe('navigate');
    }
  });

  it('rejects invalid JSON', () => {
    const result = parseCommand('not json');
    expect(result.success).toBe(false);
    expect(result.error).toBe('Invalid JSON');
  });

  it('rejects unknown action', () => {
    const input = JSON.stringify({ id: '1', action: 'fly' });
    const result = parseCommand(input);
    expect(result.success).toBe(false);
  });

  it('parses snapshot with interactive flag', () => {
    const input = JSON.stringify({ id: '1', action: 'snapshot', interactive: true });
    const result = parseCommand(input);
    expect(result.success).toBe(true);
  });

  it('parses click with @ref selector', () => {
    const input = JSON.stringify({ id: '1', action: 'click', selector: '@e1' });
    const result = parseCommand(input);
    expect(result.success).toBe(true);
  });

  // CloakBrowser-specific: stealth launch with fingerprint options
  it('parses launch with stealth options', () => {
    const input = JSON.stringify({
      id: '1',
      action: 'launch',
      headless: true,
      proxy: { server: 'http://proxy:8080' },
      geoip: true,
      fingerprintSeed: 42,
    });
    const result = parseCommand(input);
    expect(result.success).toBe(true);
  });
});

describe('responses', () => {
  it('creates success response', () => {
    const resp = successResponse('1', { text: 'hello' });
    expect(resp).toEqual({ id: '1', success: true, data: { text: 'hello' } });
  });

  it('creates error response', () => {
    const resp = errorResponse('1', 'something broke');
    expect(resp).toEqual({ id: '1', success: false, error: 'something broke' });
  });

  it('serializes to JSON string', () => {
    const resp = successResponse('1', 'ok');
    const json = serializeResponse(resp);
    expect(JSON.parse(json)).toEqual(resp);
  });
});
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/ashrafali/cloak-agent && npx vitest run tests/protocol.test.ts`
Expected: FAIL — module not found

**Step 3: Implement protocol.ts**

```ts
// src/lib/protocol.ts
import { z } from 'zod';

// --- Base ---
const base = z.object({ id: z.string(), action: z.string() });

// --- Launch (CloakBrowser-enhanced) ---
const launchSchema = base.extend({
  action: z.literal('launch'),
  headless: z.boolean().optional(),
  viewport: z.object({ width: z.number(), height: z.number() }).optional(),
  executablePath: z.string().optional(),
  proxy: z.union([
    z.string(),
    z.object({
      server: z.string(),
      bypass: z.string().optional(),
      username: z.string().optional(),
      password: z.string().optional(),
    }),
  ]).optional(),
  args: z.array(z.string()).optional(),
  userAgent: z.string().optional(),
  extensions: z.array(z.string()).optional(),
  profile: z.string().optional(),
  storageState: z.string().optional(),
  ignoreHTTPSErrors: z.boolean().optional(),
  // CloakBrowser stealth options
  geoip: z.boolean().optional(),
  fingerprintSeed: z.number().optional(),
  timezone: z.string().optional(),
  locale: z.string().optional(),
  platform: z.enum(['windows', 'macos', 'linux']).optional(),
  gpuVendor: z.string().optional(),
  gpuRenderer: z.string().optional(),
});

// --- Navigation ---
const navigateSchema = base.extend({
  action: z.literal('navigate'),
  url: z.string().min(1),
  waitUntil: z.enum(['load', 'domcontentloaded', 'networkidle']).optional(),
});

// --- Interactions ---
const clickSchema = base.extend({
  action: z.literal('click'),
  selector: z.string().min(1),
  button: z.enum(['left', 'right', 'middle']).optional(),
  clickCount: z.number().optional(),
  delay: z.number().optional(),
});

const fillSchema = base.extend({
  action: z.literal('fill'),
  selector: z.string().min(1),
  value: z.string(),
});

const typeSchema = base.extend({
  action: z.literal('type'),
  selector: z.string().min(1),
  text: z.string(),
  delay: z.number().optional(),
  clear: z.boolean().optional(),
});

const checkSchema = base.extend({ action: z.literal('check'), selector: z.string().min(1) });
const uncheckSchema = base.extend({ action: z.literal('uncheck'), selector: z.string().min(1) });
const hoverSchema = base.extend({ action: z.literal('hover'), selector: z.string().min(1) });
const focusSchema = base.extend({ action: z.literal('focus'), selector: z.string().min(1) });
const dblclickSchema = base.extend({ action: z.literal('dblclick'), selector: z.string().min(1) });

const selectSchema = base.extend({
  action: z.literal('select'),
  selector: z.string().min(1),
  values: z.union([z.string(), z.array(z.string())]),
});

const uploadSchema = base.extend({
  action: z.literal('upload'),
  selector: z.string().min(1),
  files: z.union([z.string(), z.array(z.string())]),
});

const dragSchema = base.extend({
  action: z.literal('drag'),
  source: z.string().min(1),
  target: z.string().min(1),
});

// --- Keyboard ---
const pressSchema = base.extend({
  action: z.literal('press'),
  key: z.string().min(1),
  selector: z.string().optional(),
});

const keyDownSchema = base.extend({ action: z.literal('keydown'), key: z.string().min(1) });
const keyUpSchema = base.extend({ action: z.literal('keyup'), key: z.string().min(1) });

// --- Snapshot ---
const snapshotSchema = base.extend({
  action: z.literal('snapshot'),
  interactive: z.boolean().optional(),
  maxDepth: z.number().optional(),
  compact: z.boolean().optional(),
  selector: z.string().optional(),
});

// --- Screenshot / PDF ---
const screenshotSchema = base.extend({
  action: z.literal('screenshot'),
  path: z.string().nullable().optional(),
  fullPage: z.boolean().optional(),
  selector: z.string().optional(),
  format: z.enum(['png', 'jpeg']).optional(),
  quality: z.number().optional(),
});

const pdfSchema = base.extend({
  action: z.literal('pdf'),
  path: z.string().min(1),
  format: z.enum(['Letter', 'Legal', 'Tabloid', 'A3', 'A4', 'A5']).optional(),
});

// --- Evaluate ---
const evaluateSchema = base.extend({
  action: z.literal('evaluate'),
  script: z.string().min(1),
  args: z.array(z.unknown()).optional(),
});

// --- Wait ---
const waitSchema = base.extend({
  action: z.literal('wait'),
  selector: z.string().optional(),
  timeout: z.number().optional(),
  state: z.enum(['attached', 'detached', 'visible', 'hidden']).optional(),
});

const waitForUrlSchema = base.extend({
  action: z.literal('waitforurl'),
  url: z.string().min(1),
  timeout: z.number().optional(),
});

const waitForLoadStateSchema = base.extend({
  action: z.literal('waitforloadstate'),
  state: z.enum(['load', 'domcontentloaded', 'networkidle']),
  timeout: z.number().optional(),
});

const waitForFunctionSchema = base.extend({
  action: z.literal('waitforfunction'),
  expression: z.string().min(1),
  timeout: z.number().optional(),
});

// --- Scroll ---
const scrollSchema = base.extend({
  action: z.literal('scroll'),
  selector: z.string().optional(),
  direction: z.enum(['up', 'down', 'left', 'right']).optional(),
  amount: z.number().optional(),
  x: z.number().optional(),
  y: z.number().optional(),
});

const scrollIntoViewSchema = base.extend({
  action: z.literal('scrollintoview'),
  selector: z.string().min(1),
});

// --- Navigation controls ---
const backSchema = base.extend({ action: z.literal('back') });
const forwardSchema = base.extend({ action: z.literal('forward') });
const reloadSchema = base.extend({ action: z.literal('reload') });
const closeSchema = base.extend({ action: z.literal('close') });
const urlSchema = base.extend({ action: z.literal('url') });
const titleSchema = base.extend({ action: z.literal('title') });

// --- Element info ---
const getTextSchema = base.extend({ action: z.literal('gettext'), selector: z.string().min(1) });
const innerHtmlSchema = base.extend({ action: z.literal('innerhtml'), selector: z.string().min(1) });
const inputValueSchema = base.extend({ action: z.literal('inputvalue'), selector: z.string().min(1) });
const getAttributeSchema = base.extend({
  action: z.literal('getattribute'),
  selector: z.string().min(1),
  attribute: z.string().min(1),
});
const isVisibleSchema = base.extend({ action: z.literal('isvisible'), selector: z.string().min(1) });
const isEnabledSchema = base.extend({ action: z.literal('isenabled'), selector: z.string().min(1) });
const isCheckedSchema = base.extend({ action: z.literal('ischecked'), selector: z.string().min(1) });
const countSchema = base.extend({ action: z.literal('count'), selector: z.string().min(1) });
const boundingBoxSchema = base.extend({ action: z.literal('boundingbox'), selector: z.string().min(1) });

// --- Tabs ---
const tabNewSchema = base.extend({ action: z.literal('tab_new'), url: z.string().optional() });
const tabListSchema = base.extend({ action: z.literal('tab_list') });
const tabSwitchSchema = base.extend({ action: z.literal('tab_switch'), index: z.number() });
const tabCloseSchema = base.extend({ action: z.literal('tab_close'), index: z.number().optional() });

// --- Cookies & Storage ---
const cookiesGetSchema = base.extend({ action: z.literal('cookies_get'), urls: z.array(z.string()).optional() });
const cookiesSetSchema = base.extend({
  action: z.literal('cookies_set'),
  cookies: z.array(z.object({
    name: z.string(), value: z.string(),
    url: z.string().optional(), domain: z.string().optional(),
    path: z.string().optional(), expires: z.number().optional(),
    httpOnly: z.boolean().optional(), secure: z.boolean().optional(),
    sameSite: z.enum(['Strict', 'Lax', 'None']).optional(),
  })),
});
const cookiesClearSchema = base.extend({ action: z.literal('cookies_clear') });

const storageGetSchema = base.extend({
  action: z.literal('storage_get'),
  type: z.enum(['local', 'session']),
  key: z.string().optional(),
});
const storageSetSchema = base.extend({
  action: z.literal('storage_set'),
  type: z.enum(['local', 'session']),
  key: z.string().min(1),
  value: z.string(),
});
const storageClearSchema = base.extend({
  action: z.literal('storage_clear'),
  type: z.enum(['local', 'session']),
});

// --- Dialog ---
const dialogSchema = base.extend({
  action: z.literal('dialog'),
  response: z.enum(['accept', 'dismiss']),
  promptText: z.string().optional(),
});

// --- Network ---
const routeSchema = base.extend({
  action: z.literal('route'),
  url: z.string().min(1),
  response: z.object({
    status: z.number().optional(),
    body: z.string().optional(),
    contentType: z.string().optional(),
    headers: z.record(z.string()).optional(),
  }).optional(),
  abort: z.boolean().optional(),
});
const unrouteSchema = base.extend({ action: z.literal('unroute'), url: z.string().optional() });
const requestsSchema = base.extend({
  action: z.literal('requests'),
  filter: z.string().optional(),
  clear: z.boolean().optional(),
});

// --- Settings ---
const viewportSchema = base.extend({ action: z.literal('viewport'), width: z.number(), height: z.number() });
const deviceSchema = base.extend({ action: z.literal('device'), device: z.string().min(1) });
const geolocationSchema = base.extend({
  action: z.literal('geolocation'),
  latitude: z.number(), longitude: z.number(), accuracy: z.number().optional(),
});
const headersSchema = base.extend({ action: z.literal('headers'), headers: z.record(z.string()) });
const credentialsSchema = base.extend({
  action: z.literal('credentials'),
  username: z.string(), password: z.string(),
});
const offlineSchema = base.extend({ action: z.literal('offline'), offline: z.boolean() });
const emulateMediaSchema = base.extend({
  action: z.literal('emulatemedia'),
  media: z.enum(['screen', 'print']).nullable().optional(),
  colorScheme: z.enum(['light', 'dark', 'no-preference']).nullable().optional(),
  reducedMotion: z.enum(['reduce', 'no-preference']).nullable().optional(),
});

// --- State ---
const stateSaveSchema = base.extend({ action: z.literal('state_save'), path: z.string().min(1) });
const stateLoadSchema = base.extend({ action: z.literal('state_load'), path: z.string().min(1) });

// --- Debug ---
const consoleSchema = base.extend({ action: z.literal('console'), clear: z.boolean().optional() });
const errorsSchema = base.extend({ action: z.literal('errors'), clear: z.boolean().optional() });
const highlightSchema = base.extend({ action: z.literal('highlight'), selector: z.string().min(1) });

// --- Trace / Recording ---
const traceStartSchema = base.extend({ action: z.literal('trace_start') });
const traceStopSchema = base.extend({ action: z.literal('trace_stop'), path: z.string().min(1) });
const recordingStartSchema = base.extend({
  action: z.literal('recording_start'),
  path: z.string().min(1),
  url: z.string().optional(),
});
const recordingStopSchema = base.extend({ action: z.literal('recording_stop') });

// --- Semantic locators ---
const getByRoleSchema = base.extend({
  action: z.literal('getbyrole'),
  role: z.string().min(1),
  name: z.string().optional(),
  subaction: z.enum(['click', 'fill', 'check', 'hover']),
  value: z.string().optional(),
});
const getByTextSchema = base.extend({
  action: z.literal('getbytext'),
  text: z.string().min(1),
  exact: z.boolean().optional(),
  subaction: z.enum(['click', 'hover']),
});
const getByLabelSchema = base.extend({
  action: z.literal('getbylabel'),
  label: z.string().min(1),
  subaction: z.enum(['click', 'fill', 'check']),
  value: z.string().optional(),
});

// --- Mouse ---
const mouseMoveSchema = base.extend({ action: z.literal('mousemove'), x: z.number(), y: z.number() });
const mouseDownSchema = base.extend({ action: z.literal('mousedown'), button: z.enum(['left', 'right', 'middle']).optional() });
const mouseUpSchema = base.extend({ action: z.literal('mouseup'), button: z.enum(['left', 'right', 'middle']).optional() });
const wheelSchema = base.extend({
  action: z.literal('wheel'),
  deltaX: z.number().optional(),
  deltaY: z.number().optional(),
  selector: z.string().optional(),
});

// ========== CLOAK-AGENT EXCLUSIVE COMMANDS ==========

// --- Stealth status: run detection tests ---
const stealthStatusSchema = base.extend({
  action: z.literal('stealth_status'),
});

// --- Fingerprint rotation mid-session ---
const fingerprintRotateSchema = base.extend({
  action: z.literal('fingerprint_rotate'),
  seed: z.number().optional(),
});

// --- Profile management ---
const profileCreateSchema = base.extend({
  action: z.literal('profile_create'),
  name: z.string().min(1),
});
const profileListSchema = base.extend({
  action: z.literal('profile_list'),
});

// ========== UNION ==========

const commandSchema = z.discriminatedUnion('action', [
  launchSchema, navigateSchema,
  clickSchema, fillSchema, typeSchema, checkSchema, uncheckSchema,
  hoverSchema, focusSchema, dblclickSchema, selectSchema, uploadSchema, dragSchema,
  pressSchema, keyDownSchema, keyUpSchema,
  snapshotSchema, screenshotSchema, pdfSchema,
  evaluateSchema,
  waitSchema, waitForUrlSchema, waitForLoadStateSchema, waitForFunctionSchema,
  scrollSchema, scrollIntoViewSchema,
  backSchema, forwardSchema, reloadSchema, closeSchema, urlSchema, titleSchema,
  getTextSchema, innerHtmlSchema, inputValueSchema, getAttributeSchema,
  isVisibleSchema, isEnabledSchema, isCheckedSchema, countSchema, boundingBoxSchema,
  tabNewSchema, tabListSchema, tabSwitchSchema, tabCloseSchema,
  cookiesGetSchema, cookiesSetSchema, cookiesClearSchema,
  storageGetSchema, storageSetSchema, storageClearSchema,
  dialogSchema,
  routeSchema, unrouteSchema, requestsSchema,
  viewportSchema, deviceSchema, geolocationSchema, headersSchema, credentialsSchema,
  offlineSchema, emulateMediaSchema,
  stateSaveSchema, stateLoadSchema,
  consoleSchema, errorsSchema, highlightSchema,
  traceStartSchema, traceStopSchema, recordingStartSchema, recordingStopSchema,
  getByRoleSchema, getByTextSchema, getByLabelSchema,
  mouseMoveSchema, mouseDownSchema, mouseUpSchema, wheelSchema,
  // Cloak-agent exclusive
  stealthStatusSchema, fingerprintRotateSchema,
  profileCreateSchema, profileListSchema,
]);

// --- Parse / Serialize ---

export type Command = z.infer<typeof commandSchema>;

export interface SuccessResponse { id: string; success: true; data: unknown; }
export interface ErrorResponse { id: string; success: false; error: string; }
export type Response = SuccessResponse | ErrorResponse;

export interface ParseSuccess { success: true; command: Command; }
export interface ParseFailure { success: false; error: string; id?: string; }
export type ParseResult = ParseSuccess | ParseFailure;

export function parseCommand(input: string): ParseResult {
  let json: unknown;
  try {
    json = JSON.parse(input);
  } catch {
    return { success: false, error: 'Invalid JSON' };
  }

  const id = typeof json === 'object' && json !== null && 'id' in json
    ? String((json as Record<string, unknown>).id)
    : undefined;

  const result = commandSchema.safeParse(json);
  if (!result.success) {
    const errors = result.error.errors.map(e => `${e.path.join('.')}: ${e.message}`).join(', ');
    return { success: false, error: `Validation error: ${errors}`, id };
  }

  return { success: true, command: result.data };
}

export function successResponse(id: string, data: unknown): SuccessResponse {
  return { id, success: true, data };
}

export function errorResponse(id: string, error: string): ErrorResponse {
  return { id, success: false, error };
}

export function serializeResponse(response: Response): string {
  return JSON.stringify(response);
}
```

**Step 4: Run tests**

Run: `npx vitest run tests/protocol.test.ts`
Expected: All 7 tests PASS

**Step 5: Commit**

```bash
git add src/lib/protocol.ts tests/protocol.test.ts
git commit -m "feat: protocol layer with Zod schemas including stealth commands"
```

---

## Task 3: Snapshot Engine (ARIA Tree + @ref IDs)

**Files:**
- Create: `src/lib/snapshot.ts`
- Create: `tests/snapshot.test.ts`

**Step 1: Write failing tests**

```ts
// tests/snapshot.test.ts
import { describe, it, expect } from 'vitest';
import {
  processAriaTree,
  parseRef,
  getSnapshotStats,
  resetRefs,
} from '../src/lib/snapshot.js';

describe('parseRef', () => {
  it('parses @e1 format', () => {
    expect(parseRef('@e1')).toBe('e1');
  });
  it('parses ref=e1 format', () => {
    expect(parseRef('ref=e5')).toBe('e5');
  });
  it('parses bare e1 format', () => {
    expect(parseRef('e3')).toBe('e3');
  });
  it('returns null for CSS selectors', () => {
    expect(parseRef('#my-button')).toBeNull();
  });
});

describe('processAriaTree', () => {
  const sampleTree = `- heading "Example Domain" [level=1]
- paragraph: Some text
- button "Submit"
- textbox "Email"
- link "Click here"`;

  it('adds refs to interactive elements in interactive mode', () => {
    resetRefs();
    const refs: Record<string, unknown> = {};
    const result = processAriaTree(sampleTree, refs, { interactive: true });
    expect(result).toContain('[ref=e1]');
    expect(result).toContain('button "Submit"');
    expect(result).toContain('textbox "Email"');
    expect(result).toContain('link "Click here"');
    expect(result).not.toContain('paragraph');
    expect(Object.keys(refs).length).toBe(3);
  });

  it('adds refs to all relevant elements in full mode', () => {
    resetRefs();
    const refs: Record<string, unknown> = {};
    const result = processAriaTree(sampleTree, refs, {});
    expect(result).toContain('[ref=');
    expect(result).toContain('heading');
    expect(result).toContain('paragraph');
  });
});

describe('getSnapshotStats', () => {
  it('returns token estimate', () => {
    const stats = getSnapshotStats('hello world test', { e1: { role: 'button' } });
    expect(stats.tokens).toBeGreaterThan(0);
    expect(stats.refs).toBe(1);
    expect(stats.chars).toBe(16);
  });
});
```

**Step 2: Run tests to verify they fail**

Run: `npx vitest run tests/snapshot.test.ts`
Expected: FAIL

**Step 3: Implement snapshot.ts**

Port agent-browser's snapshot.ts with these additions:
- Export `processAriaTree` and `resetRefs` for testability
- Add token count in stats output
- Same INTERACTIVE_ROLES, CONTENT_ROLES, STRUCTURAL_ROLES sets
- Same ref assignment and deduplication logic

(Full implementation: replicate the logic from agent-browser's snapshot.ts — the functions `resetRefs`, `nextRef`, `buildSelector`, `getEnhancedSnapshot`, `processAriaTree`, `processLine`, `compactTree`, `parseRef`, `getSnapshotStats`, `removeNthFromNonDuplicates`, `createRoleNameTracker`. These are pure functions with no external dependencies beyond Playwright's ariaSnapshot API.)

**Step 4: Run tests**

Run: `npx vitest run tests/snapshot.test.ts`
Expected: All PASS

**Step 5: Commit**

```bash
git add src/lib/snapshot.ts tests/snapshot.test.ts
git commit -m "feat: snapshot engine with ARIA tree refs and token counting"
```

---

## Task 4: AI-Friendly Error Messages

**Files:**
- Create: `src/lib/errors.ts`
- Create: `tests/errors.test.ts`

**Step 1: Write failing tests**

```ts
// tests/errors.test.ts
import { describe, it, expect } from 'vitest';
import { toAIFriendlyError } from '../src/lib/errors.js';

describe('toAIFriendlyError', () => {
  it('handles strict mode violation', () => {
    const err = new Error('strict mode violation: resolved to 3 elements');
    const result = toAIFriendlyError(err, '@e1');
    expect(result.message).toContain('matched 3 elements');
    expect(result.message).toContain('snapshot');
  });

  it('handles element blocked by overlay', () => {
    const err = new Error('Element intercepts pointer events');
    const result = toAIFriendlyError(err, '@e1');
    expect(result.message).toContain('blocked');
    expect(result.message).toContain('modal');
  });

  it('handles timeout', () => {
    const err = new Error('Timeout 30000ms exceeded');
    const result = toAIFriendlyError(err, '@e1');
    expect(result.message).toContain('timed out');
  });

  it('passes through unknown errors', () => {
    const err = new Error('something weird');
    const result = toAIFriendlyError(err, '@e1');
    expect(result.message).toBe('something weird');
  });
});
```

**Step 2: Implement errors.ts**

```ts
// src/lib/errors.ts
export function toAIFriendlyError(error: unknown, selector?: string): Error {
  const message = error instanceof Error ? error.message : String(error);

  if (message.includes('strict mode violation')) {
    const countMatch = message.match(/resolved to (\d+) elements/);
    const count = countMatch ? countMatch[1] : 'multiple';
    return new Error(
      `Selector "${selector}" matched ${count} elements. ` +
      `Run 'snapshot' to get updated refs, or use a more specific selector.`
    );
  }

  if (message.includes('intercepts pointer events')) {
    return new Error(
      `Element "${selector}" is blocked by another element (likely a modal or overlay). ` +
      `Try dismissing any modals/cookie banners first.`
    );
  }

  if (message.includes('not visible') && !message.includes('Timeout')) {
    return new Error(
      `Element "${selector}" is not visible. Try scrolling it into view or check if it's hidden.`
    );
  }

  if (message.includes('Timeout') && message.includes('exceeded')) {
    return new Error(
      `Action on "${selector}" timed out. The element may be blocked, still loading, or not interactable. ` +
      `Run 'snapshot' to check the current page state.`
    );
  }

  if (message.includes('waiting for') && (message.includes('to be visible') || message.includes('Timeout'))) {
    return new Error(
      `Element "${selector}" not found or not visible. Run 'snapshot' to see current page elements.`
    );
  }

  return error instanceof Error ? error : new Error(message);
}
```

**Step 3: Run tests**

Run: `npx vitest run tests/errors.test.ts`
Expected: All PASS

**Step 4: Commit**

```bash
git add src/lib/errors.ts tests/errors.test.ts
git commit -m "feat: AI-friendly error messages for common Playwright failures"
```

---

## Task 5: Stealth Module (CloakBrowser Integration)

**Files:**
- Create: `src/lib/stealth.ts`
- Create: `tests/stealth.test.ts`

**Step 1: Write failing tests**

```ts
// tests/stealth.test.ts
import { describe, it, expect } from 'vitest';
import {
  buildStealthArgs,
  getProfileDir,
  listProfiles,
  getDefaultStealthConfig,
} from '../src/lib/stealth.js';

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

  it('sets macos platform on darwin', () => {
    const args = buildStealthArgs({ platform: 'macos' });
    expect(args).toContain('--fingerprint-platform=macos');
    expect(args.some(a => a.includes('Apple'))).toBe(true);
  });

  it('sets windows platform with NVIDIA GPU', () => {
    const args = buildStealthArgs({ platform: 'windows' });
    expect(args).toContain('--fingerprint-platform=windows');
    expect(args.some(a => a.includes('NVIDIA'))).toBe(true);
  });

  it('allows custom GPU vendor/renderer', () => {
    const args = buildStealthArgs({ gpuVendor: 'AMD', gpuRenderer: 'Radeon RX 7900' });
    expect(args).toContain('--fingerprint-gpu-vendor=AMD');
    expect(args).toContain('--fingerprint-gpu-renderer=Radeon RX 7900');
  });

  it('merges user args without duplicating keys', () => {
    const args = buildStealthArgs({ args: ['--no-sandbox', '--custom-flag=true'] });
    const sandboxCount = args.filter(a => a.startsWith('--no-sandbox')).length;
    expect(sandboxCount).toBe(1);
    expect(args).toContain('--custom-flag=true');
  });
});

describe('getDefaultStealthConfig', () => {
  it('returns config object with expected keys', () => {
    const config = getDefaultStealthConfig();
    expect(config).toHaveProperty('viewport');
    expect(config).toHaveProperty('platform');
    expect(config.viewport.width).toBe(1920);
  });
});

describe('profiles', () => {
  it('getProfileDir returns path under ~/.cloak-agent/profiles', () => {
    const dir = getProfileDir('test-profile');
    expect(dir).toContain('.cloak-agent');
    expect(dir).toContain('profiles');
    expect(dir).toContain('test-profile');
  });
});
```

**Step 2: Implement stealth.ts**

```ts
// src/lib/stealth.ts
import os from 'node:os';
import path from 'node:path';
import fs from 'node:fs';

export interface StealthOptions {
  fingerprintSeed?: number;
  platform?: 'windows' | 'macos' | 'linux';
  gpuVendor?: string;
  gpuRenderer?: string;
  timezone?: string;
  locale?: string;
  args?: string[];
}

export interface StealthConfig {
  viewport: { width: number; height: number };
  platform: string;
}

const DATA_DIR = path.join(os.homedir(), '.cloak-agent');
const PROFILES_DIR = path.join(DATA_DIR, 'profiles');

export function getDefaultStealthConfig(): StealthConfig {
  const isMac = process.platform === 'darwin';
  return {
    viewport: { width: 1920, height: 947 },
    platform: isMac ? 'macos' : 'windows',
  };
}

export function buildStealthArgs(options: StealthOptions): string[] {
  const seed = options.fingerprintSeed ?? Math.floor(Math.random() * 90000) + 10000;
  const platform = options.platform ?? (process.platform === 'darwin' ? 'macos' : 'windows');

  const seen = new Map<string, string>();

  // Base stealth args
  const base = [
    '--no-sandbox',
    '--disable-blink-features=AutomationControlled',
    `--fingerprint=${seed}`,
    `--fingerprint-platform=${platform}`,
  ];

  for (const arg of base) {
    seen.set(arg.split('=')[0], arg);
  }

  // GPU spoofing
  if (options.gpuVendor) {
    seen.set('--fingerprint-gpu-vendor', `--fingerprint-gpu-vendor=${options.gpuVendor}`);
  } else if (platform === 'macos') {
    seen.set('--fingerprint-gpu-vendor', '--fingerprint-gpu-vendor=Google Inc. (Apple)');
  } else {
    seen.set('--fingerprint-gpu-vendor', '--fingerprint-gpu-vendor=NVIDIA Corporation');
  }

  if (options.gpuRenderer) {
    seen.set('--fingerprint-gpu-renderer', `--fingerprint-gpu-renderer=${options.gpuRenderer}`);
  } else if (platform === 'macos') {
    seen.set('--fingerprint-gpu-renderer',
      '--fingerprint-gpu-renderer=ANGLE (Apple, ANGLE Metal Renderer: Apple M3, Unspecified Version)');
  } else {
    seen.set('--fingerprint-gpu-renderer', '--fingerprint-gpu-renderer=NVIDIA GeForce RTX 3070');
  }

  // Timezone
  if (options.timezone) {
    seen.set('--fingerprint-timezone', `--fingerprint-timezone=${options.timezone}`);
  }

  // Locale
  if (options.locale) {
    seen.set('--lang', `--lang=${options.locale}`);
  }

  // User args (override defaults)
  if (options.args) {
    for (const arg of options.args) {
      seen.set(arg.split('=')[0], arg);
    }
  }

  return [...seen.values()];
}

export function getProfileDir(name: string): string {
  return path.join(PROFILES_DIR, name);
}

export function listProfiles(): string[] {
  if (!fs.existsSync(PROFILES_DIR)) return [];
  return fs.readdirSync(PROFILES_DIR, { withFileTypes: true })
    .filter(d => d.isDirectory())
    .map(d => d.name);
}

export function ensureProfileDir(name: string): string {
  const dir = getProfileDir(name);
  if (!fs.existsSync(dir)) {
    fs.mkdirSync(dir, { recursive: true });
  }
  return dir;
}
```

**Step 3: Run tests**

Run: `npx vitest run tests/stealth.test.ts`
Expected: All PASS

**Step 4: Commit**

```bash
git add src/lib/stealth.ts tests/stealth.test.ts
git commit -m "feat: stealth module with fingerprint args, GPU spoofing, profile management"
```

---

## Task 6: BrowserManager (CloakBrowser Launch + Playwright Control)

**Files:**
- Create: `src/lib/browser.ts`
- Create: `tests/browser.test.ts`

This is the core module. It wraps CloakBrowser's `launch()`, `launchContext()`, and `launchPersistentContext()` and adds:
- The snapshot ref map (like agent-browser's BrowserManager)
- Tab/page management
- Console/error tracking
- Network route tracking
- Dialog handling
- Recording/tracing
- Screencast via CDP

**Step 1: Write failing tests** (unit tests for non-browser methods)

```ts
// tests/browser.test.ts
import { describe, it, expect } from 'vitest';
import { BrowserManager } from '../src/lib/browser.js';

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
    expect(bm.getRefMap()).toEqual({});
  });
});
```

**Step 2: Implement browser.ts**

Key structure (abbreviated — full implementation follows agent-browser's BrowserManager pattern but uses CloakBrowser for launch):

```ts
// src/lib/browser.ts
import { type Browser, type BrowserContext, type Page, type Frame, devices } from 'playwright-core';
import { launch as cloakLaunch, launchPersistentContext as cloakLaunchPersistent, ensureBinary } from 'cloakbrowser';
import { getEnhancedSnapshot, parseRef as snapshotParseRef } from './snapshot.js';
import { buildStealthArgs, getProfileDir, ensureProfileDir, type StealthOptions } from './stealth.js';

export interface LaunchOptions extends StealthOptions {
  headless?: boolean;
  proxy?: string | { server: string; bypass?: string; username?: string; password?: string };
  geoip?: boolean;
  profile?: string;
  storageState?: string;
  executablePath?: string;
  extensions?: string[];
  userAgent?: string;
  viewport?: { width: number; height: number };
  ignoreHTTPSErrors?: boolean;
}

export interface RefData {
  selector: string;
  role: string;
  name?: string;
  nth?: number;
}

export class BrowserManager {
  private browser: Browser | null = null;
  private contexts: BrowserContext[] = [];
  private pages: Page[] = [];
  private activePageIndex = 0;
  private activeFrame: Frame | null = null;
  private refMap: Record<string, RefData> = {};
  private lastSnapshot = '';
  private consoleMessages: Array<{ type: string; text: string }> = [];
  private pageErrors: string[] = [];
  private trackedRequests: Array<{ url: string; method: string; status?: number }> = [];
  private routes = new Map<string, unknown>();
  private isPersistentContext = false;

  isLaunched(): boolean {
    return this.browser !== null || this.isPersistentContext;
  }

  resolveRef(ref: string): RefData | null {
    return this.refMap[ref] ?? null;
  }

  getRefMap(): Record<string, RefData> {
    return { ...this.refMap };
  }

  async launch(options: LaunchOptions = {}): Promise<void> {
    await ensureBinary();

    if (options.profile) {
      // Use persistent context for profile-based sessions
      const userDataDir = ensureProfileDir(options.profile);
      const args = buildStealthArgs(options);
      const { chromium } = await import('playwright-core');
      const binaryPath = process.env.CLOAKBROWSER_BINARY_PATH || (await ensureBinary());

      const context = await chromium.launchPersistentContext(userDataDir, {
        executablePath: binaryPath,
        headless: options.headless ?? true,
        args,
        ignoreDefaultArgs: ['--enable-automation'],
        ...(options.proxy ? {
          proxy: typeof options.proxy === 'string'
            ? { server: options.proxy }
            : options.proxy
        } : {}),
        ...(options.userAgent ? { userAgent: options.userAgent } : {}),
        viewport: options.viewport ?? { width: 1920, height: 947 },
        ...(options.ignoreHTTPSErrors ? { ignoreHTTPSErrors: true } : {}),
      });

      this.isPersistentContext = true;
      this.contexts = [context];
      this.pages = context.pages().length > 0 ? [context.pages()[0]] : [await context.newPage()];
      this.setupPageListeners(this.pages[0]);
      return;
    }

    // Standard launch via CloakBrowser
    this.browser = await cloakLaunch({
      headless: options.headless ?? true,
      proxy: options.proxy,
      geoip: options.geoip,
      timezone: options.timezone,
      locale: options.locale,
      args: buildStealthArgs(options),
      ...(options.ignoreHTTPSErrors ? { launchOptions: { ignoreHTTPSErrors: true } } : {}),
    });

    const context = await this.browser.newContext({
      viewport: options.viewport ?? { width: 1920, height: 947 },
      ...(options.userAgent ? { userAgent: options.userAgent } : {}),
      ...(options.storageState ? { storageState: options.storageState } : {}),
    });

    this.contexts = [context];
    const page = await context.newPage();
    this.pages = [page];
    this.setupPageListeners(page);
  }

  private setupPageListeners(page: Page): void {
    page.on('console', msg => {
      this.consoleMessages.push({ type: msg.type(), text: msg.text() });
    });
    page.on('pageerror', err => {
      this.pageErrors.push(err.message);
    });
  }

  getPage(): Page {
    if (this.pages.length === 0) throw new Error('No page available. Launch browser first.');
    return this.pages[this.activePageIndex];
  }

  getFrame(): Frame {
    return this.activeFrame ?? this.getPage().mainFrame();
  }

  async getSnapshot(options: { interactive?: boolean; maxDepth?: number; compact?: boolean; selector?: string } = {}) {
    const page = this.getPage();
    const result = await getEnhancedSnapshot(page, options);
    this.refMap = result.refs as Record<string, RefData>;
    this.lastSnapshot = result.tree;
    return result;
  }

  async close(): Promise<void> {
    for (const context of this.contexts) {
      await context.close().catch(() => {});
    }
    if (this.browser) {
      await this.browser.close().catch(() => {});
    }
    this.browser = null;
    this.contexts = [];
    this.pages = [];
    this.refMap = {};
    this.isPersistentContext = false;
  }

  // ... Additional methods for tabs, network, dialogs, etc.
  // These follow agent-browser's BrowserManager pattern exactly.
}
```

**Step 3: Run tests**

Run: `npx vitest run tests/browser.test.ts`
Expected: All PASS

**Step 4: Commit**

```bash
git add src/lib/browser.ts tests/browser.test.ts
git commit -m "feat: BrowserManager with CloakBrowser launch and snapshot integration"
```

---

## Task 7: Action Executor

**Files:**
- Create: `src/lib/actions.ts`

This maps every protocol command to a BrowserManager method call. Same switch/case pattern as agent-browser's actions.ts but calls our BrowserManager and adds handlers for:
- `stealth_status` — navigates to bot detection sites, returns pass/fail
- `fingerprint_rotate` — closes and relaunches with new seed
- `profile_create` / `profile_list`

**Step 1: Implement actions.ts**

The executor function signature:

```ts
export async function executeCommand(command: Command, browser: BrowserManager): Promise<Response>
```

Each case:
- `launch` → `browser.launch(command)`
- `navigate` → `browser.getPage().goto(command.url, ...)`
- `click` → resolve ref or use selector, then `locator.click()`
- `snapshot` → `browser.getSnapshot(command)` → return tree + stats
- `stealth_status` → navigate to `https://bot.sannysoft.com`, extract results
- `fingerprint_rotate` → close browser, relaunch with new seed
- `profile_create` → `ensureProfileDir(command.name)`
- `profile_list` → `listProfiles()`
- All other commands follow agent-browser's action handlers

**Step 2: Commit**

```bash
git add src/lib/actions.ts
git commit -m "feat: action executor with stealth-specific commands"
```

---

## Task 8: Daemon (Unix Socket Server)

**Files:**
- Create: `src/lib/daemon.ts`

Port agent-browser's daemon.ts with these changes:
- Uses our BrowserManager (which uses CloakBrowser internally)
- Data directory: `~/.cloak-agent/` instead of `~/.agent-browser/`
- Socket files: `~/.cloak-agent/<session>.sock`
- Reads `CLOAK_AGENT_*` env vars instead of `AGENT_BROWSER_*`
- Same security: rejects HTTP requests on the socket
- Same auto-launch behavior

**Step 1: Implement daemon.ts**

Same structure as agent-browser's daemon.ts but with our namespace:

```ts
// src/lib/daemon.ts
import * as net from 'node:net';
import * as fs from 'node:fs';
import * as path from 'node:path';
import * as os from 'node:os';
import { BrowserManager } from './browser.js';
import { parseCommand, serializeResponse, errorResponse } from './protocol.js';
import { executeCommand } from './actions.js';

const isWindows = process.platform === 'win32';
let currentSession = process.env.CLOAK_AGENT_SESSION || 'default';

export function getAppDir(): string {
  if (process.env.XDG_RUNTIME_DIR) {
    return path.join(process.env.XDG_RUNTIME_DIR, 'cloak-agent');
  }
  return path.join(os.homedir(), '.cloak-agent');
}

// ... getSocketDir, getSocketPath, getPidFile, isDaemonRunning,
// cleanupSocket, startDaemon — same pattern as agent-browser
// but with 'cloak-agent' namespace and CLOAK_AGENT_ env prefix
```

**Step 2: Commit**

```bash
git add src/lib/daemon.ts
git commit -m "feat: daemon server with Unix socket and session support"
```

---

## Task 9: CLI Parser (Command Line → JSON)

**Files:**
- Create: `src/cli/parser.ts`
- Create: `tests/cli-parser.test.ts`

**Step 1: Write failing tests**

```ts
// tests/cli-parser.test.ts
import { describe, it, expect } from 'vitest';
import { parseArgs } from '../src/cli/parser.js';

describe('parseArgs', () => {
  it('parses: open https://example.com', () => {
    const cmd = parseArgs(['open', 'https://example.com']);
    expect(cmd.action).toBe('navigate');
    expect(cmd.url).toBe('https://example.com');
  });

  it('parses: snapshot -i', () => {
    const cmd = parseArgs(['snapshot', '-i']);
    expect(cmd.action).toBe('snapshot');
    expect(cmd.interactive).toBe(true);
  });

  it('parses: click @e1', () => {
    const cmd = parseArgs(['click', '@e1']);
    expect(cmd.action).toBe('click');
    expect(cmd.selector).toBe('@e1');
  });

  it('parses: fill @e2 "hello"', () => {
    const cmd = parseArgs(['fill', '@e2', 'hello']);
    expect(cmd.action).toBe('fill');
    expect(cmd.selector).toBe('@e2');
    expect(cmd.value).toBe('hello');
  });

  it('parses: stealth status', () => {
    const cmd = parseArgs(['stealth', 'status']);
    expect(cmd.action).toBe('stealth_status');
  });

  it('parses: fingerprint rotate', () => {
    const cmd = parseArgs(['fingerprint', 'rotate']);
    expect(cmd.action).toBe('fingerprint_rotate');
  });

  it('parses: fingerprint rotate --seed 42', () => {
    const cmd = parseArgs(['fingerprint', 'rotate', '--seed', '42']);
    expect(cmd.action).toBe('fingerprint_rotate');
    expect(cmd.seed).toBe(42);
  });

  it('parses: profile create myprofile', () => {
    const cmd = parseArgs(['profile', 'create', 'myprofile']);
    expect(cmd.action).toBe('profile_create');
    expect(cmd.name).toBe('myprofile');
  });

  it('parses: profile list', () => {
    const cmd = parseArgs(['profile', 'list']);
    expect(cmd.action).toBe('profile_list');
  });

  it('parses: close', () => {
    const cmd = parseArgs(['close']);
    expect(cmd.action).toBe('close');
  });

  it('parses: screenshot --full path.png', () => {
    const cmd = parseArgs(['screenshot', '--full', 'path.png']);
    expect(cmd.action).toBe('screenshot');
    expect(cmd.fullPage).toBe(true);
    expect(cmd.path).toBe('path.png');
  });

  it('parses: get text @e1', () => {
    const cmd = parseArgs(['get', 'text', '@e1']);
    expect(cmd.action).toBe('gettext');
    expect(cmd.selector).toBe('@e1');
  });
});
```

**Step 2: Implement parser.ts**

Maps CLI verbs to JSON command objects. Same verb-noun pattern as agent-browser:

```
open <url>           → { action: 'navigate', url }
snapshot [-i] [-c]   → { action: 'snapshot', interactive, compact }
click @ref           → { action: 'click', selector }
fill @ref "text"     → { action: 'fill', selector, value }
stealth status       → { action: 'stealth_status' }
fingerprint rotate   → { action: 'fingerprint_rotate' }
profile create <n>   → { action: 'profile_create', name }
profile list         → { action: 'profile_list' }
get text @ref        → { action: 'gettext', selector }
...etc
```

**Step 3: Run tests**

Run: `npx vitest run tests/cli-parser.test.ts`
Expected: All PASS

**Step 4: Commit**

```bash
git add src/cli/parser.ts tests/cli-parser.test.ts
git commit -m "feat: CLI parser mapping commands to protocol JSON"
```

---

## Task 10: CLI Entrypoint (bin/cloak-agent.js)

**Files:**
- Create: `bin/cloak-agent.js`

**Step 1: Implement the CLI entrypoint**

```js
#!/usr/bin/env node

// bin/cloak-agent.js
// Sends parsed CLI commands to the daemon via Unix socket.
// Auto-starts daemon if not running.

import * as net from 'node:net';
import { randomUUID } from 'node:crypto';
import { spawn } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

async function main() {
  const args = process.argv.slice(2);

  // Handle --session flag
  let session = process.env.CLOAK_AGENT_SESSION || 'default';
  const sessionIdx = args.indexOf('--session');
  if (sessionIdx !== -1 && args[sessionIdx + 1]) {
    session = args.splice(sessionIdx, 2)[1];
  }

  // Handle --json flag
  const jsonMode = args.includes('--json');
  if (jsonMode) args.splice(args.indexOf('--json'), 1);

  // Handle --headed flag
  // Handle --timeout flag
  // ... (pass through to launch command)

  // Dynamic import to allow tree-shaking
  const { parseArgs } = await import('../dist/cli/parser.js');
  const { getSocketPath, isDaemonRunning, setSession } = await import('../dist/lib/daemon.js');

  setSession(session);

  // Auto-start daemon if not running
  if (!isDaemonRunning(session)) {
    const daemonPath = path.join(__dirname, '..', 'dist', 'lib', 'daemon.js');
    const child = spawn(process.execPath, [daemonPath], {
      env: { ...process.env, CLOAK_AGENT_SESSION: session, CLOAK_AGENT_DAEMON: '1' },
      detached: true,
      stdio: 'ignore',
    });
    child.unref();
    // Wait for socket to appear
    await waitForSocket(getSocketPath(session));
  }

  // Parse CLI args to command JSON
  const command = parseArgs(args);
  command.id = randomUUID().slice(0, 8);

  // Send to daemon
  const response = await sendCommand(getSocketPath(session), JSON.stringify(command));
  const parsed = JSON.parse(response);

  if (jsonMode) {
    process.stdout.write(response + '\n');
  } else if (parsed.success) {
    if (typeof parsed.data === 'string') {
      process.stdout.write(parsed.data + '\n');
    } else if (parsed.data != null) {
      process.stdout.write(JSON.stringify(parsed.data, null, 2) + '\n');
    }
  } else {
    process.stderr.write(`Error: ${parsed.error}\n`);
    process.exit(1);
  }
}

function sendCommand(socketPath, data) {
  return new Promise((resolve, reject) => {
    const socket = net.createConnection(socketPath);
    let buffer = '';
    socket.on('connect', () => socket.write(data + '\n'));
    socket.on('data', chunk => { buffer += chunk.toString(); });
    socket.on('end', () => resolve(buffer.trim()));
    socket.on('error', reject);
  });
}

function waitForSocket(socketPath, timeout = 5000) {
  const start = Date.now();
  return new Promise((resolve, reject) => {
    const check = () => {
      const socket = net.createConnection(socketPath);
      socket.on('connect', () => { socket.destroy(); resolve(undefined); });
      socket.on('error', () => {
        if (Date.now() - start > timeout) reject(new Error('Daemon failed to start'));
        else setTimeout(check, 50);
      });
    };
    check();
  });
}

main().catch(err => {
  process.stderr.write(`${err.message}\n`);
  process.exit(1);
});
```

**Step 2: Make executable**

Run: `chmod +x bin/cloak-agent.js`

**Step 3: Commit**

```bash
git add bin/cloak-agent.js
git commit -m "feat: CLI entrypoint with auto-daemon launch and socket communication"
```

---

## Task 11: WebSocket Stream Server

**Files:**
- Create: `src/lib/stream-server.ts`

Port agent-browser's stream-server.ts as-is. This provides:
- WebSocket server for live viewport streaming
- Screencast via CDP
- Input injection (mouse/keyboard/touch) from remote clients
- Origin-based security (reject browser-origin connections)

**Step 1: Implement**

Same as agent-browser's StreamServer class. No changes needed — it operates on the BrowserManager interface which we match.

**Step 2: Commit**

```bash
git add src/lib/stream-server.ts
git commit -m "feat: WebSocket stream server for viewport streaming"
```

---

## Task 12: Public API (index.ts)

**Files:**
- Create: `src/index.ts`

```ts
// src/index.ts
export { BrowserManager } from './lib/browser.js';
export type { LaunchOptions, RefData } from './lib/browser.js';
export { buildStealthArgs, getDefaultStealthConfig, listProfiles } from './lib/stealth.js';
export { parseCommand, successResponse, errorResponse, serializeResponse } from './lib/protocol.js';
export type { Command, Response } from './lib/protocol.js';
export { getEnhancedSnapshot, parseRef, getSnapshotStats } from './lib/snapshot.js';
export { toAIFriendlyError } from './lib/errors.js';
```

**Step 1: Commit**

```bash
git add src/index.ts
git commit -m "feat: public API exports"
```

---

## Task 13: Integration Test

**Files:**
- Create: `tests/integration.test.ts`

```ts
// tests/integration.test.ts
import { describe, it, expect, afterAll } from 'vitest';
import { BrowserManager } from '../src/lib/browser.js';

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
  });

  it('takes interactive snapshot with refs', async () => {
    const { tree, refs } = await browser.getSnapshot({ interactive: true });
    expect(tree).toContain('[ref=');
    expect(Object.keys(refs).length).toBeGreaterThan(0);
  });

  it('snapshot includes token estimate in stats', async () => {
    const { tree, refs } = await browser.getSnapshot({ interactive: true });
    const { getSnapshotStats } = await import('../src/lib/snapshot.js');
    const stats = getSnapshotStats(tree, refs);
    expect(stats.tokens).toBeGreaterThan(0);
    expect(stats.tokens).toBeLessThan(500); // example.com is tiny
  });
}, { timeout: 60000 });
```

**Step 1: Run integration tests**

Run: `npx vitest run tests/integration.test.ts`
Expected: All PASS (first run downloads ~200MB Chromium binary)

**Step 2: Commit**

```bash
git add tests/integration.test.ts
git commit -m "test: integration test for stealth browser launch and snapshot"
```

---

## Task 14: Build, Link, and Smoke Test

**Step 1: Build TypeScript**

Run: `npm run build`
Expected: `dist/` directory created with .js and .d.ts files

**Step 2: Link globally**

Run: `npm link`
Expected: `cloak-agent` command available globally

**Step 3: Smoke test CLI**

```bash
cloak-agent open https://example.com
cloak-agent snapshot -i
cloak-agent get title
cloak-agent stealth status
cloak-agent close
```

Expected: Each command returns output. Snapshot shows refs. Stealth status shows detection results.

**Step 4: Commit**

```bash
git add -A
git commit -m "feat: cloak-agent v0.1.0 — stealth browser CLI for AI agents"
```

---

## Summary: What Cloak-Agent Gets You vs. Each Tool Alone

| Feature | agent-browser | CloakBrowser | cloak-agent |
|---------|:---:|:---:|:---:|
| Token-efficient ARIA snapshots | Y | N | Y |
| @ref element targeting | Y | N | Y |
| CLI-first (one command = one action) | Y | N | Y |
| Daemon architecture (persistent session) | Y | N | Y |
| Source-level stealth Chromium | N | Y | Y |
| Fingerprint randomization | N | Y | Y |
| GPU/platform spoofing | N | Y | Y |
| GeoIP proxy auto-detection | N | Y | Y |
| Persistent profiles (anti-incognito) | N | Y | Y |
| `stealth status` detection check | N | N | Y |
| `fingerprint rotate` mid-session | N | N | Y |
| Named profile management | N | N | Y |
| AI-friendly errors | Y | N | Y |
| WebSocket viewport streaming | Y | N | Y |
