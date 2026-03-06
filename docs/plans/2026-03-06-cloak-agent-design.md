# Cloak Agent Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build `cloak-agent` — a stealth browser automation CLI for AI agents. Go CLI front-end (fast, single binary, cross-platform) + Node.js daemon back-end (Playwright + CloakBrowser's stealth Chromium). Agent-first design following the principles from [Rewrite Your CLI for AI Agents](https://justin.poehnelt.com/posts/rewrite-your-cli-for-ai-agents/) and [Ship Types, Not Docs](https://shiptypes.com).

**Architecture:** Two-process split — same proven pattern as agent-browser (Vercel):
1. **Go CLI binary** — sub-millisecond startup, parses args to JSON, sends over Unix socket, prints response. Single binary, zero runtime dependencies for the caller. Ships with `--json` raw payload mode, runtime schema introspection (`cloak-agent schema <command>`), input hardening, and `--dry-run` for mutations.
2. **Node.js daemon** — long-running process managing CloakBrowser's stealth Chromium via Playwright. Owns the browser lifecycle, ARIA snapshot engine, @ref system, action execution, and WebSocket streaming. Auto-spawned by the Go CLI on first command.

**Tech Stack:**
- CLI: Go 1.22+, `encoding/json`, `net` (Unix socket), `os/exec` (daemon spawn)
- Daemon: Node.js, TypeScript (ESM), playwright-core, cloakbrowser, zod, ws
- Testing: `go test` (CLI), vitest (daemon)
- Build: `go build` (CLI), `tsc` (daemon)

---

## Design Principles (from studied sources)

### From "Rewrite Your CLI for AI Agents" (Justin Poehnelt)

1. **Raw JSON payloads over custom flags** — Support `--json '{...}'` for any command so agents can pass the full API payload. Human-friendly flags too, but JSON is first-class.

2. **Runtime schema introspection** — `cloak-agent schema navigate` dumps the Zod schema as machine-readable JSON. The CLI is its own documentation.

3. **Context window discipline** — Snapshots include token count. `--fields` flag limits returned data. NDJSON mode for streaming large responses.

4. **Input hardening against hallucinations** — Reject path traversals, strip control characters, block `?#%` in resource names, validate all @refs before sending to Playwright.

5. **Agent skills as documentation** — Ship a `skills/` directory with structured Markdown (YAML frontmatter) for each command surface. Encodes invariant rules agents can't derive from `--help`.

6. **Multi-surface architecture** — CLI + MCP server mode (`cloak-agent mcp`) for JSON-RPC over stdio. Same daemon, different transport.

7. **Safety rails** — `--dry-run` validates locally before browser actions. Response sanitization to prevent prompt injection from page content.

### From "Ship Types, Not Docs" (Boris Tane)

- Protocol schemas (Zod) are the single source of truth
- The Go CLI doesn't re-implement validation — it passes raw JSON to the daemon which validates via Zod
- Schema introspection is auto-generated from the Zod definitions, never hand-maintained

### From agent-browser (Vercel) — Proven Patterns

- Daemon over Unix socket (TCP on Windows)
- ARIA accessibility tree snapshots with `@ref` IDs
- AI-friendly error messages
- Session isolation via named sockets
- Auto-launch daemon on first command

### CloakBrowser — Stealth Layer

- Source-level Chromium patches (not JS injection)
- `--fingerprint=<seed>` for deterministic fingerprint generation
- `--fingerprint-platform=`, `--fingerprint-gpu-vendor=`, `--fingerprint-gpu-renderer=`
- GeoIP auto-detection from proxy IP (timezone + locale)
- Persistent context support (avoids incognito detection)

---

## Architecture Diagram

```
┌─────────────────────────────────┐
│         Go CLI Binary           │
│  (cloak-agent)                  │
│                                 │
│  ┌─ Arg parser ────────────┐   │
│  │  open https://x.com     │   │
│  │  → {"action":"navigate",│   │
│  │     "url":"https://x.."}│   │
│  └──────────────────────────┘   │
│  ┌─ Socket client ─────────┐   │
│  │  Connect ~/.cloak-agent/ │   │
│  │  <session>.sock          │   │
│  │  Send JSON, recv JSON    │   │
│  └──────────────────────────┘   │
│  ┌─ Daemon launcher ───────┐   │
│  │  Spawn node daemon.js   │   │
│  │  if not running          │   │
│  └──────────────────────────┘   │
│  ┌─ Output formatter ──────┐   │
│  │  --json → raw JSON       │   │
│  │  default → human-friendly│   │
│  │  --fields → filtered     │   │
│  └──────────────────────────┘   │
└─────────────────────────────────┘
            │ Unix Socket / TCP
            ▼
┌─────────────────────────────────┐
│       Node.js Daemon            │
│  (daemon.js — long-running)     │
│                                 │
│  ┌─ Protocol ──────────────┐   │
│  │  Zod validation         │   │
│  │  JSON parse/serialize   │   │
│  │  Schema introspection   │   │
│  └──────────────────────────┘   │
│  ┌─ BrowserManager ────────┐   │
│  │  CloakBrowser launch()  │   │
│  │  Stealth Chromium binary│   │
│  │  Fingerprint args       │   │
│  │  GeoIP proxy detection  │   │
│  │  Persistent profiles    │   │
│  └──────────────────────────┘   │
│  ┌─ Snapshot Engine ───────┐   │
│  │  ARIA tree → @ref IDs   │   │
│  │  Interactive/compact    │   │
│  │  Token count estimate   │   │
│  └──────────────────────────┘   │
│  ┌─ Action Executor ───────┐   │
│  │  70+ commands            │   │
│  │  + stealth_status        │   │
│  │  + fingerprint_rotate    │   │
│  │  + profile_create/list   │   │
│  │  AI-friendly errors      │   │
│  └──────────────────────────┘   │
│  ┌─ Stream Server ─────────┐   │
│  │  WebSocket viewport     │   │
│  │  Screencast + input     │   │
│  └──────────────────────────┘   │
└─────────────────────────────────┘
```

---

## File Structure

```
cloak-agent/
├── go.mod                        # Go module
├── go.sum
├── main.go                       # CLI entrypoint
├── cmd/
│   ├── root.go                   # Top-level command dispatch
│   ├── client.go                 # Unix socket client + daemon launcher
│   ├── parser.go                 # CLI args → JSON command
│   ├── output.go                 # Response formatting (human/json/fields)
│   └── schema.go                 # `schema` subcommand (calls daemon for Zod dump)
├── daemon/
│   ├── package.json
│   ├── tsconfig.json
│   ├── vitest.config.ts
│   ├── src/
│   │   ├── daemon.ts             # Socket server, session management
│   │   ├── browser.ts            # BrowserManager (CloakBrowser + Playwright)
│   │   ├── snapshot.ts           # ARIA snapshot engine with @refs
│   │   ├── actions.ts            # Command executor (all browser actions)
│   │   ├── protocol.ts           # Zod schemas, parse/serialize, schema export
│   │   ├── stealth.ts            # Fingerprint args, GPU spoof, profiles, detection
│   │   ├── errors.ts             # AI-friendly error messages
│   │   ├── stream-server.ts      # WebSocket viewport streaming
│   │   └── index.ts              # Programmatic API exports
│   └── tests/
│       ├── protocol.test.ts
│       ├── snapshot.test.ts
│       ├── stealth.test.ts
│       ├── errors.test.ts
│       └── integration.test.ts
├── skills/                        # Agent skill files (structured markdown)
│   ├── navigation.md
│   ├── interaction.md
│   ├── stealth.md
│   └── snapshots.md
├── scripts/
│   ├── build.sh                  # Build Go CLI + TS daemon
│   └── install.sh                # Install globally
├── tests/
│   ├── parser_test.go
│   ├── client_test.go
│   └── output_test.go
└── docs/
    └── plans/
        └── 2026-03-06-cloak-agent-design.md
```

---

## Task 1: Go Project Scaffolding

**Files:**
- Create: `go.mod`
- Create: `main.go`
- Create: `cmd/root.go`
- Create: `.gitignore`

**Step 1: Initialize Go module**

Run: `cd /Users/ashrafali/cloak-agent && go mod init github.com/ashrafali/cloak-agent`
Expected: `go.mod` created

**Step 2: Create main.go**

```go
// main.go
package main

import (
	"fmt"
	"os"

	"github.com/ashrafali/cloak-agent/cmd"
)

func main() {
	if err := cmd.Execute(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}
```

**Step 3: Create cmd/root.go (stub)**

```go
// cmd/root.go
package cmd

import "fmt"

func Execute(args []string) error {
	if len(args) == 0 {
		fmt.Println("cloak-agent — stealth browser automation CLI for AI agents")
		fmt.Println("Usage: cloak-agent <command> [args...]")
		return nil
	}
	return fmt.Errorf("unknown command: %s", args[0])
}
```

**Step 4: Create .gitignore**

```
# Go
cloak-agent
*.exe

# Daemon
daemon/node_modules/
daemon/dist/
daemon/*.tsbuildinfo

# OS
.DS_Store
```

**Step 5: Verify it builds**

Run: `go build -o cloak-agent . && ./cloak-agent`
Expected: Prints usage message

**Step 6: Commit**

```bash
git add go.mod main.go cmd/root.go .gitignore
git commit -m "feat: Go project scaffolding with CLI entrypoint"
```

---

## Task 2: Daemon Project Scaffolding

**Files:**
- Create: `daemon/package.json`
- Create: `daemon/tsconfig.json`
- Create: `daemon/vitest.config.ts`

**Step 1: Create daemon/package.json**

```json
{
  "name": "cloak-agent-daemon",
  "version": "0.1.0",
  "private": true,
  "type": "module",
  "main": "dist/daemon.js",
  "scripts": {
    "build": "tsc",
    "test": "vitest run",
    "test:watch": "vitest",
    "dev": "tsc --watch"
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
  }
}
```

**Step 2: Create daemon/tsconfig.json**

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "module": "NodeNext",
    "moduleResolution": "NodeNext",
    "outDir": "dist",
    "rootDir": "src",
    "declaration": true,
    "sourceMap": true,
    "strict": true,
    "esModuleInterop": true,
    "skipLibCheck": true
  },
  "include": ["src/**/*"],
  "exclude": ["node_modules", "dist", "tests"]
}
```

**Step 3: Create daemon/vitest.config.ts**

```ts
import { defineConfig } from 'vitest/config';
export default defineConfig({ test: { globals: true, testTimeout: 30000 } });
```

**Step 4: Install daemon dependencies**

Run: `cd /Users/ashrafali/cloak-agent/daemon && npm install`
Expected: `added N packages`

**Step 5: Commit**

```bash
git add daemon/package.json daemon/tsconfig.json daemon/vitest.config.ts
git commit -m "feat: daemon scaffolding with TypeScript, CloakBrowser, Playwright deps"
```

---

## Task 3: Daemon Protocol Layer

**Files:**
- Create: `daemon/src/protocol.ts`
- Create: `daemon/tests/protocol.test.ts`

The protocol layer defines all command schemas in Zod and exports a `dumpSchemas()` function that serializes them as JSON for the Go CLI's `schema` subcommand. This is the "ship types, not docs" principle — the Zod schemas are the canonical API definition.

**Step 1: Write failing tests**

```ts
// daemon/tests/protocol.test.ts
import { describe, it, expect } from 'vitest';
import { parseCommand, successResponse, errorResponse, serializeResponse, dumpSchema } from '../src/protocol.js';

describe('parseCommand', () => {
  it('parses a valid navigate command', () => {
    const result = parseCommand(JSON.stringify({ id: '1', action: 'navigate', url: 'https://example.com' }));
    expect(result.success).toBe(true);
  });

  it('rejects invalid JSON', () => {
    const result = parseCommand('not json');
    expect(result.success).toBe(false);
    expect(result.error).toBe('Invalid JSON');
  });

  it('rejects unknown action', () => {
    const result = parseCommand(JSON.stringify({ id: '1', action: 'fly' }));
    expect(result.success).toBe(false);
  });

  it('parses launch with stealth options', () => {
    const result = parseCommand(JSON.stringify({
      id: '1', action: 'launch', headless: true,
      proxy: { server: 'http://proxy:8080' },
      geoip: true, fingerprintSeed: 42,
    }));
    expect(result.success).toBe(true);
  });

  it('parses raw JSON payload via --json flag semantics', () => {
    // Agents send the full payload — no translation needed
    const result = parseCommand(JSON.stringify({
      id: '1', action: 'navigate', url: 'https://example.com', waitUntil: 'networkidle',
    }));
    expect(result.success).toBe(true);
    if (result.success) {
      expect(result.command.action).toBe('navigate');
    }
  });

  it('validates and rejects path traversal in file paths', () => {
    const result = parseCommand(JSON.stringify({
      id: '1', action: 'screenshot', path: '../../.ssh/id_rsa',
    }));
    // The protocol layer should accept this — input hardening happens in actions
    // But the schema should still parse it
    expect(result.success).toBe(true);
  });

  // Cloak-exclusive commands
  it('parses stealth_status', () => {
    const result = parseCommand(JSON.stringify({ id: '1', action: 'stealth_status' }));
    expect(result.success).toBe(true);
  });

  it('parses fingerprint_rotate', () => {
    const result = parseCommand(JSON.stringify({ id: '1', action: 'fingerprint_rotate', seed: 99999 }));
    expect(result.success).toBe(true);
  });

  it('parses profile_create', () => {
    const result = parseCommand(JSON.stringify({ id: '1', action: 'profile_create', name: 'myprofile' }));
    expect(result.success).toBe(true);
  });
});

describe('dumpSchema', () => {
  it('returns JSON schema for a known action', () => {
    const schema = dumpSchema('navigate');
    expect(schema).toHaveProperty('action');
    expect(schema).toHaveProperty('url');
  });

  it('returns null for unknown action', () => {
    const schema = dumpSchema('fly');
    expect(schema).toBeNull();
  });
});

describe('responses', () => {
  it('creates success response', () => {
    const resp = successResponse('1', { text: 'hello' });
    expect(resp).toEqual({ id: '1', success: true, data: { text: 'hello' } });
  });

  it('serializes to JSON string', () => {
    const json = serializeResponse(successResponse('1', 'ok'));
    expect(JSON.parse(json).success).toBe(true);
  });
});
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/ashrafali/cloak-agent/daemon && npx vitest run tests/protocol.test.ts`
Expected: FAIL — module not found

**Step 3: Implement protocol.ts**

Same comprehensive Zod schemas as the previous plan, but add:
- `dumpSchema(action: string)` — returns a simplified JSON representation of a single action's schema
- `dumpAllSchemas()` — returns all schemas as a map
- The launch schema includes CloakBrowser stealth options: `geoip`, `fingerprintSeed`, `timezone`, `locale`, `platform`, `gpuVendor`, `gpuRenderer`
- New exclusive commands: `stealth_status`, `fingerprint_rotate`, `profile_create`, `profile_list`

The `dumpSchema` function uses Zod's `.shape` property to extract field names/types and serialize them as JSON — this powers the Go CLI's `cloak-agent schema <command>` introspection.

**Step 4: Run tests, verify pass**

Run: `cd /Users/ashrafali/cloak-agent/daemon && npx vitest run tests/protocol.test.ts`
Expected: All PASS

**Step 5: Commit**

```bash
git add daemon/src/protocol.ts daemon/tests/protocol.test.ts
git commit -m "feat: protocol layer with Zod schemas, stealth commands, schema introspection"
```

---

## Task 4: Daemon Snapshot Engine

**Files:**
- Create: `daemon/src/snapshot.ts`
- Create: `daemon/tests/snapshot.test.ts`

Same ARIA tree → @ref engine from agent-browser. Port the logic:
- `resetRefs()`, `nextRef()` — counter for `e1`, `e2`, ...
- `INTERACTIVE_ROLES`, `CONTENT_ROLES`, `STRUCTURAL_ROLES` — role classification
- `processAriaTree()` — parse Playwright's ariaSnapshot(), inject `[ref=eN]`
- `getEnhancedSnapshot(page, options)` — main entry point
- `parseRef(arg)` — parse `@e1`, `ref=e1`, `e1` formats
- `getSnapshotStats(tree, refs)` — line count, char count, token estimate, ref count

Token estimate: `Math.ceil(chars / 4)` — same heuristic, exposed in every snapshot response.

**Step 1: Write tests** (same as previous plan's snapshot tests)

**Step 2: Implement** (port from agent-browser's snapshot.ts)

**Step 3: Run tests, verify pass**

**Step 4: Commit**

```bash
git add daemon/src/snapshot.ts daemon/tests/snapshot.test.ts
git commit -m "feat: ARIA snapshot engine with @ref IDs and token counting"
```

---

## Task 5: Daemon AI-Friendly Errors

**Files:**
- Create: `daemon/src/errors.ts`
- Create: `daemon/tests/errors.test.ts`

Port from agent-browser. Translate Playwright errors into actionable suggestions:
- Strict mode violation → "matched N elements, run snapshot"
- Intercepts pointer → "blocked by overlay, dismiss modals"
- Timeout → "timed out, run snapshot to check state"
- Not visible → "try scrolling into view"

**Plus input hardening** (from Poehnelt's article):
- `validateFilePath(path)` — reject `../`, normalize, sandbox to allowed dirs
- `sanitizeInput(text)` — strip ASCII control chars < 0x20 (except newline/tab)
- `validateRef(ref)` — must match `/^e\d+$/` after parsing

**Step 1-4: Test, implement, verify, commit**

```bash
git commit -m "feat: AI-friendly errors and input hardening against hallucinations"
```

---

## Task 6: Daemon Stealth Module

**Files:**
- Create: `daemon/src/stealth.ts`
- Create: `daemon/tests/stealth.test.ts`

CloakBrowser integration layer:
- `buildStealthArgs(options)` — merge stealth defaults + user overrides
- `getProfileDir(name)` / `listProfiles()` / `ensureProfileDir(name)` — persistent profiles under `~/.cloak-agent/profiles/`
- `getDefaultStealthConfig()` — platform-aware defaults (macOS Apple GPU, Windows NVIDIA)
- `checkStealthStatus(page)` — navigate to detection sites, extract pass/fail results

**Step 1-4: Test, implement, verify, commit**

```bash
git commit -m "feat: stealth module — fingerprint args, profiles, detection check"
```

---

## Task 7: Daemon BrowserManager

**Files:**
- Create: `daemon/src/browser.ts`
- Create: `daemon/tests/browser.test.ts`

Core browser lifecycle manager. Uses CloakBrowser's `launch()` internally:
- `launch(options)` — calls `cloakLaunch()` with stealth args, sets up context/page
- `launch()` with `profile` option — uses `launchPersistentContext()` for anti-incognito
- `getSnapshot(options)` — calls snapshot engine, caches refMap
- `resolveRef(ref)` → Playwright locator from refMap
- Tab management (new, list, switch, close)
- Console/error/request tracking
- Dialog handling
- Recording/tracing via CDP
- `close()` — clean teardown

**Step 1-4: Test, implement, verify, commit**

```bash
git commit -m "feat: BrowserManager with CloakBrowser launch and ref resolution"
```

---

## Task 8: Daemon Action Executor

**Files:**
- Create: `daemon/src/actions.ts`

Maps every protocol command to BrowserManager calls. ~70 standard commands (from agent-browser) plus 4 exclusive:

- `stealth_status` → navigate to `https://bot.sannysoft.com`, extract table, return pass/fail JSON
- `fingerprint_rotate` → save cookies/state, close browser, relaunch with new seed, restore state
- `profile_create` → `ensureProfileDir(name)`, return path
- `profile_list` → `listProfiles()`, return array

Input hardening applied here:
- File paths validated via `validateFilePath()` before any fs operation
- @refs validated before lookup
- URLs checked for basic validity

`--dry-run` support: if `command.dryRun === true`, validate the command and return what would happen without executing.

**Step 1: Implement, commit**

```bash
git commit -m "feat: action executor with 70+ commands, stealth exclusives, dry-run"
```

---

## Task 9: Daemon Server (Unix Socket)

**Files:**
- Create: `daemon/src/daemon.ts`

Socket server managing sessions:
- Socket dir: `~/.cloak-agent/` (or `XDG_RUNTIME_DIR/cloak-agent/`)
- Socket files: `<session>.sock`, `<session>.pid`, `<session>.stream`
- Env prefix: `CLOAK_AGENT_*`
- HTTP request rejection (security)
- Auto-launch browser on first non-launch command
- Special handler for `schema` action — returns `dumpSchema()` / `dumpAllSchemas()` without needing browser
- Graceful shutdown on SIGINT/SIGTERM

**Step 1: Implement, commit**

```bash
git commit -m "feat: daemon Unix socket server with session management"
```

---

## Task 10: Daemon WebSocket Stream Server

**Files:**
- Create: `daemon/src/stream-server.ts`

Port from agent-browser. Provides:
- WebSocket server for live viewport streaming (screencast via CDP)
- Input injection (mouse/keyboard/touch)
- Origin-based security

**Step 1: Implement, commit**

```bash
git commit -m "feat: WebSocket stream server for viewport streaming"
```

---

## Task 11: Daemon Public API + Build

**Files:**
- Create: `daemon/src/index.ts`

Export all public APIs for programmatic use. Then build:

Run: `cd /Users/ashrafali/cloak-agent/daemon && npm run build`
Expected: `dist/` created with all .js and .d.ts files

**Step 1: Implement, build, commit**

```bash
git commit -m "feat: daemon public API exports and first successful build"
```

---

## Task 12: Go CLI — Socket Client

**Files:**
- Create: `cmd/client.go`
- Create: `tests/client_test.go`

The socket client:
- Connects to `~/.cloak-agent/<session>.sock`
- Sends JSON command + `\n`
- Reads JSON response until `\n`
- Timeout support (default 30s, configurable via `--timeout`)
- Returns parsed response struct

```go
type Response struct {
    ID      string      `json:"id"`
    Success bool        `json:"success"`
    Data    interface{} `json:"data,omitempty"`
    Error   string      `json:"error,omitempty"`
}
```

**Step 1: Write failing test**

```go
// tests/client_test.go
func TestGetSocketPath(t *testing.T) {
    path := cmd.GetSocketPath("default")
    if !strings.Contains(path, ".cloak-agent") {
        t.Errorf("expected .cloak-agent in path, got %s", path)
    }
    if !strings.HasSuffix(path, "default.sock") {
        t.Errorf("expected default.sock suffix, got %s", path)
    }
}
```

**Step 2: Implement, test, commit**

```bash
git commit -m "feat: Go socket client with daemon auto-launch"
```

---

## Task 13: Go CLI — Arg Parser

**Files:**
- Create: `cmd/parser.go`
- Create: `tests/parser_test.go`

Maps CLI args to JSON command objects. Supports two modes:

**Mode 1: Human-friendly flags**
```
cloak-agent open https://example.com
cloak-agent snapshot -i
cloak-agent click @e1
cloak-agent fill @e2 "hello"
cloak-agent stealth status
cloak-agent fingerprint rotate --seed 42
cloak-agent profile create myprofile
```

**Mode 2: Raw JSON payload (agent-first)**
```
cloak-agent --json '{"action":"navigate","url":"https://example.com","waitUntil":"networkidle"}'
```

The parser also handles global flags:
- `--session <name>` — named session
- `--json` — JSON output mode (when used without payload)
- `--json '{...}'` — raw payload mode (when followed by JSON string)
- `--timeout <ms>` — command timeout
- `--headed` — show browser window
- `--dry-run` — validate without executing
- `--fields <list>` — limit returned fields (context window discipline)

**Step 1: Write failing tests**

```go
// tests/parser_test.go
func TestParseOpen(t *testing.T) {
    cmd := parser.Parse([]string{"open", "https://example.com"})
    if cmd["action"] != "navigate" { t.Fatal("expected navigate") }
    if cmd["url"] != "https://example.com" { t.Fatal("wrong url") }
}

func TestParseSnapshot(t *testing.T) {
    cmd := parser.Parse([]string{"snapshot", "-i"})
    if cmd["action"] != "snapshot" { t.Fatal("expected snapshot") }
    if cmd["interactive"] != true { t.Fatal("expected interactive=true") }
}

func TestParseRawJSON(t *testing.T) {
    cmd := parser.ParseRawJSON(`{"action":"navigate","url":"https://x.com"}`)
    if cmd["action"] != "navigate" { t.Fatal("expected navigate") }
}

func TestParseStealthStatus(t *testing.T) {
    cmd := parser.Parse([]string{"stealth", "status"})
    if cmd["action"] != "stealth_status" { t.Fatal("expected stealth_status") }
}

func TestParseFingerprintRotate(t *testing.T) {
    cmd := parser.Parse([]string{"fingerprint", "rotate", "--seed", "42"})
    if cmd["action"] != "fingerprint_rotate" { t.Fatal("expected fingerprint_rotate") }
    if cmd["seed"] != float64(42) { t.Fatal("expected seed=42") }
}

func TestParseDryRun(t *testing.T) {
    cmd, flags := parser.ParseWithFlags([]string{"--dry-run", "open", "https://x.com"})
    if !flags.DryRun { t.Fatal("expected dry-run") }
    if cmd["action"] != "navigate" { t.Fatal("expected navigate") }
}
```

**Step 2: Implement, test, commit**

```bash
git commit -m "feat: Go CLI arg parser with human flags and raw JSON mode"
```

---

## Task 14: Go CLI — Output Formatter

**Files:**
- Create: `cmd/output.go`
- Create: `tests/output_test.go`

Formats daemon responses for stdout/stderr:

- **Default mode** — human-readable: snapshot trees printed as-is, strings printed raw, objects pretty-printed
- **`--json` mode** — raw JSON on stdout (one line)
- **`--fields` mode** — filter response object to only requested fields before output
- **Error responses** — printed to stderr with `Error: ` prefix, exit code 1

**Step 1: Write tests, implement, commit**

```bash
git commit -m "feat: output formatter with JSON, human, and fields modes"
```

---

## Task 15: Go CLI — Schema Subcommand

**Files:**
- Create: `cmd/schema.go`

`cloak-agent schema` — runtime schema introspection:
- `cloak-agent schema navigate` → sends `{"action":"schema","command":"navigate"}` to daemon, prints JSON schema
- `cloak-agent schema --all` → dumps all schemas
- `cloak-agent schema --list` → lists all available actions

This enables AI agents to self-discover the CLI's capabilities without static docs.

**Step 1: Implement, commit**

```bash
git commit -m "feat: schema introspection subcommand for agent self-discovery"
```

---

## Task 16: Go CLI — Root Command Wiring

**Files:**
- Modify: `cmd/root.go`
- Modify: `main.go`

Wire everything together:
1. Parse global flags (`--session`, `--json`, `--timeout`, `--headed`, `--dry-run`, `--fields`)
2. Check for raw JSON mode (`--json '{...}'`)
3. If raw JSON, send directly to daemon
4. Otherwise, parse CLI args to JSON command via parser
5. Add `id` field (random 8-char UUID)
6. If `--dry-run`, add `dryRun: true` to command
7. Send to daemon via socket client (auto-launch if needed)
8. Format and print response

Also handle:
- `cloak-agent --version` — print CLI version + daemon version
- `cloak-agent --help` — print usage
- `cloak-agent install` — download CloakBrowser binary + install daemon deps
- `cloak-agent session list` — list active sessions

**Step 1: Implement, commit**

```bash
git commit -m "feat: root command wiring — full CLI dispatch"
```

---

## Task 17: Agent Skill Files

**Files:**
- Create: `skills/navigation.md`
- Create: `skills/interaction.md`
- Create: `skills/stealth.md`
- Create: `skills/snapshots.md`

Structured Markdown with YAML frontmatter. These encode invariant rules for AI agents:

```markdown
---
name: snapshots
description: Rules for efficient page inspection
invariants:
  - Always use `snapshot -i` (interactive) unless you need full page structure
  - Always add `--fields` to limit returned data
  - Re-snapshot after any navigation or significant DOM change
  - Refs are only valid until next navigation
---

# Snapshot Commands

## Quick Reference
- `cloak-agent snapshot -i` — interactive elements only (recommended)
- `cloak-agent snapshot -c` — compact mode (less tokens)
- `cloak-agent snapshot -d 3` — limit depth

## Rules
1. **Always snapshot before interacting** — refs change on navigation
2. **Use `-i` by default** — full snapshots waste tokens
3. **Check token count** in response stats before deciding to snapshot again
```

**Step 1: Create all four skill files**

**Step 2: Commit**

```bash
git commit -m "feat: agent skill files for navigation, interaction, stealth, snapshots"
```

---

## Task 18: Build Script + Install

**Files:**
- Create: `scripts/build.sh`
- Create: `scripts/install.sh`
- Create: `Makefile`

```bash
#!/bin/bash
# scripts/build.sh
set -e

echo "Building daemon..."
cd daemon && npm run build && cd ..

echo "Building CLI..."
go build -o cloak-agent .

echo "Done. Binary: ./cloak-agent"
```

```bash
#!/bin/bash
# scripts/install.sh
set -e

./scripts/build.sh

echo "Installing CLI to /usr/local/bin..."
sudo cp cloak-agent /usr/local/bin/

echo "Installing daemon..."
mkdir -p ~/.cloak-agent/daemon
cp -r daemon/dist daemon/package.json daemon/node_modules ~/.cloak-agent/daemon/

echo "Done. Run: cloak-agent open https://example.com"
```

```makefile
# Makefile
.PHONY: build test install clean

build:
	cd daemon && npm run build
	go build -o cloak-agent .

test:
	cd daemon && npm test
	go test ./...

install: build
	./scripts/install.sh

clean:
	rm -rf cloak-agent daemon/dist
```

**Step 1: Create files, build, commit**

```bash
git commit -m "feat: build scripts and Makefile"
```

---

## Task 19: Integration Test

**Files:**
- Create: `daemon/tests/integration.test.ts`

```ts
describe('integration: stealth browser', () => {
  const browser = new BrowserManager();

  afterAll(async () => { await browser.close(); });

  it('launches CloakBrowser and navigates', async () => {
    await browser.launch({ headless: true });
    expect(browser.isLaunched()).toBe(true);
    const page = browser.getPage();
    await page.goto('https://example.com');
    expect(await page.title()).toContain('Example');
  });

  it('takes interactive snapshot with refs and token count', async () => {
    const { tree, refs } = await browser.getSnapshot({ interactive: true });
    expect(tree).toContain('[ref=');
    const { getSnapshotStats } = await import('../src/snapshot.js');
    const stats = getSnapshotStats(tree, refs);
    expect(stats.tokens).toBeGreaterThan(0);
    expect(stats.tokens).toBeLessThan(500);
  });
}, { timeout: 60000 });
```

**Step 1: Run integration tests**

Run: `cd /Users/ashrafali/cloak-agent/daemon && npx vitest run tests/integration.test.ts`
Expected: PASS (first run downloads CloakBrowser binary ~200MB)

**Step 2: Commit**

```bash
git commit -m "test: integration test — stealth launch, navigate, snapshot"
```

---

## Task 20: End-to-End Smoke Test

**Step 1: Build everything**

Run: `make build`

**Step 2: Smoke test the full CLI**

```bash
./cloak-agent open https://example.com
./cloak-agent snapshot -i
./cloak-agent get title
./cloak-agent stealth status
./cloak-agent schema navigate
./cloak-agent schema --list
./cloak-agent --json '{"action":"snapshot","interactive":true}'
./cloak-agent close
```

**Step 3: Verify `--dry-run`**

```bash
./cloak-agent --dry-run open https://example.com
# Should print: "Would navigate to https://example.com" without launching browser
```

**Step 4: Final commit**

```bash
git add -A
git commit -m "feat: cloak-agent v0.1.0 — stealth browser CLI for AI agents"
```

---

## Summary: What's Different from agent-browser

| Feature | agent-browser | cloak-agent |
|---------|:---:|:---:|
| CLI language | Rust | **Go** |
| Daemon | Node.js + Playwright | Node.js + Playwright + **CloakBrowser** |
| Stealth Chromium binary | N | **Y** |
| Fingerprint randomization | N | **Y** |
| GPU/platform spoofing | N | **Y** |
| GeoIP proxy auto-detection | N | **Y** |
| `stealth status` detection check | N | **Y** |
| `fingerprint rotate` mid-session | N | **Y** |
| Named persistent profiles | N | **Y** |
| Raw JSON payload mode (`--json '{}'`) | N | **Y** |
| Runtime schema introspection | N | **Y** |
| `--dry-run` for mutations | N | **Y** |
| `--fields` for context window control | N | **Y** |
| Input hardening (path traversal, etc) | N | **Y** |
| Agent skill files | N | **Y** |
| Token count in snapshot stats | N | **Y** |
| ARIA snapshots + @refs | Y | Y |
| Daemon architecture | Y | Y |
| Session isolation | Y | Y |
| AI-friendly errors | Y | Y |
| WebSocket viewport streaming | Y | Y |

## Sources That Informed This Design

- [Rewrite Your CLI for AI Agents](https://justin.poehnelt.com/posts/rewrite-your-cli-for-ai-agents/) — 7 principles for agent-first CLI design
- [Ship Types, Not Docs](https://shiptypes.com) — schema-first API contracts, types as documentation
- [agent-browser](https://github.com/vercel-labs/agent-browser) — proven Rust CLI + Node daemon architecture
- [CloakBrowser](https://github.com/CloakHQ/CloakBrowser) — stealth Chromium with source-level fingerprint patches
- [playwright-go](https://github.com/playwright-community/playwright-go) — Go bindings for Playwright (reference, not used directly)
