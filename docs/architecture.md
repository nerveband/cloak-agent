# Architecture

cloak-agent has two parts: a Go CLI and a Node.js daemon. They talk over a Unix socket.

## Why two processes?

CloakBrowser's stealth Chromium runs through Playwright, which is a Node.js library. But Node.js CLIs are slow to start (~100-300ms). AI agents run hundreds of commands per session — that latency adds up.

The solution: a compiled Go binary handles the CLI (sub-millisecond startup), and a long-running Node.js daemon handles the browser. The daemon starts once and stays alive, so there's no repeated startup cost.

## Go CLI

The binary at `./cloak-agent` does four things:

1. **Parses arguments** into a JSON command (`cmd/parser.go`)
2. **Connects to the daemon** over a Unix socket (`cmd/client.go`)
3. **Sends the JSON** and waits for a response
4. **Formats the response** for stdout (`cmd/output.go`)

If the daemon isn't running, the CLI starts it automatically.

The CLI has no knowledge of Playwright, browsers, or web pages. It just translates command-line arguments into JSON and prints what comes back.

### Socket location

Default: `~/.cloak-agent/default.sock`

Named sessions: `~/.cloak-agent/<session-name>.sock`

Override with `CLOAK_AGENT_SOCKET_DIR` env var.

## Node.js daemon

The daemon (`daemon/src/daemon.ts`) listens on the Unix socket and manages the browser.

### Components

**Protocol** (`protocol.ts`) — Zod schemas validate every incoming command. If the JSON doesn't match the schema, the daemon returns an error before touching the browser. The schemas also power the `schema` introspection command.

**BrowserManager** (`browser.ts`) — Wraps CloakBrowser's `launch()` function. Manages the browser instance, contexts, pages, tabs, and the ref map from snapshots.

**Snapshot engine** (`snapshot.ts`) — Calls Playwright's `ariaSnapshot()` to get the accessibility tree, then parses it and assigns `@ref` IDs to interactive elements. Returns compact text instead of HTML.

**Action executor** (`actions.ts`) — Maps each command action to the right Playwright calls. Handles 70+ commands. Wraps Playwright errors in AI-friendly messages.

**Stealth module** (`stealth.ts`) — Builds the `--fingerprint-*` CLI args for CloakBrowser's patched Chromium. Manages persistent profiles under `~/.cloak-agent/profiles/`.

**Error handler** (`errors.ts`) — Translates Playwright errors like "strict mode violation: resolved to 3 elements" into "Selector matched 3 elements. Run 'snapshot' to get updated refs." Also validates file paths and refs to block hallucinated input from agents.

**Stream server** (`stream-server.ts`) — Optional WebSocket server for live viewport streaming via CDP screencast.

### Data flow

```
Agent runs:  cloak-agent fill @e2 "hello"
                    │
                    ▼
Go CLI parses: {"action":"fill","selector":"@e2","value":"hello","id":"a1b2c3"}
                    │
                    ▼
Unix socket:  ~/.cloak-agent/default.sock
                    │
                    ▼
Daemon receives JSON, validates with Zod
                    │
                    ▼
Action executor:  resolves @e2 → getByRole('textbox', {name: "Email"})
                    │
                    ▼
Playwright:       locator.fill("hello")
                    │
                    ▼
Response:         {"id":"a1b2c3","success":true,"data":"Filled @e2 with 'hello'"}
                    │
                    ▼
Go CLI prints:    Filled @e2 with 'hello'
```

## Auto-launch

When you run any command, the Go CLI checks if a daemon is running for the current session (by looking for a PID file and verifying the process exists). If not, it:

1. Finds `daemon.js` (checks `CLOAK_AGENT_DAEMON_DIR`, then relative to the binary, then `~/.cloak-agent/daemon/`)
2. Spawns `node daemon.js` as a detached background process
3. Polls the socket every 50ms until it's ready (10s timeout)
4. Sends the command

The daemon also auto-launches the browser on the first command that needs one. So `cloak-agent open https://example.com` starts both the daemon and the browser in one step.

## Environment variables

| Variable | What it does |
|----------|-------------|
| `CLOAK_AGENT_SESSION` | Default session name |
| `CLOAK_AGENT_SOCKET_DIR` | Custom socket directory |
| `CLOAK_AGENT_DAEMON_DIR` | Custom daemon directory |
| `CLOAK_AGENT_HEADED` | Set to `1` to show browser window |
| `CLOAK_AGENT_PROXY` | Proxy server URL |
| `CLOAK_AGENT_PROXY_BYPASS` | Proxy bypass list |
| `CLOAK_AGENT_PROFILE` | Persistent profile name |
| `CLOAK_AGENT_ARGS` | Extra Chromium args (comma-separated) |
| `CLOAK_AGENT_USER_AGENT` | Custom user agent |
| `CLOAK_AGENT_STATE` | Path to storage state JSON |
| `CLOAK_AGENT_IGNORE_HTTPS_ERRORS` | Set to `1` to ignore HTTPS errors |
