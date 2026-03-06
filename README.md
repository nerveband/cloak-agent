# cloak-agent

Browser automation CLI for AI agents that websites can't detect.

Most browser automation tools get flagged by bot detection within seconds. cloak-agent doesn't. It pairs a Go CLI with [CloakBrowser](https://github.com/CloakHQ/CloakBrowser)'s stealth Chromium — a browser patched at the C++ source level so detection sites see a real browser, because it is one.

The CLI is built for AI agents, not humans. Every command returns compact text. Page snapshots use accessibility tree refs (`@e1`, `@e2`) instead of raw HTML, cutting token usage by 10-50x. Agents send one shell command per action and get back exactly what they need.

```bash
cloak-agent open https://example.com
cloak-agent snapshot -i
# - link "More information..." [ref=e1]

cloak-agent click @e1
cloak-agent stealth status
# All 30 detection tests passed
```

## How it works

Two processes, one job:

1. **Go CLI** (this binary) — parses your command, sends JSON over a Unix socket, prints the response. Sub-millisecond overhead.
2. **Node.js daemon** (runs in background) — manages the stealth Chromium browser via Playwright. Starts automatically on first command, stays alive between commands.

The daemon uses CloakBrowser's patched Chromium binary. Fingerprints (GPU, screen size, hardware profile, timezone) are randomized at the binary level — no JavaScript injection that detection scripts can catch.

## Install

### Prerequisites

- Go 1.22+
- Node.js 18+
- npm

### From source

```bash
git clone https://github.com/nerveband/cloak-agent.git
cd cloak-agent
cd daemon && npm install && cd ..
make build
```

This builds the Go binary (`./cloak-agent`) and compiles the daemon TypeScript.

### Install globally

```bash
make install
```

Copies the binary and daemon to `~/.cloak-agent/` and symlinks to `/usr/local/bin/`.

The stealth Chromium binary (~200MB) downloads automatically on first run.

## Quick start

```bash
# Navigate to a page
cloak-agent open https://example.com

# See what's on the page (interactive elements only)
cloak-agent snapshot -i

# Click something using its ref
cloak-agent click @e1

# Fill a form field
cloak-agent fill @e2 "hello@example.com"

# Check if the browser is being detected
cloak-agent stealth status

# Done — close the browser
cloak-agent close
```

## Commands

Full reference: [docs/commands.md](docs/commands.md)

### Navigation

```bash
cloak-agent open <url>          # Go to URL
cloak-agent back                # Browser back
cloak-agent forward             # Browser forward
cloak-agent reload              # Reload page
cloak-agent close               # Close browser and daemon
```

### Snapshots (the important one)

Snapshots return the page's accessibility tree with refs. This is how agents "see" the page without burning tokens on HTML.

```bash
cloak-agent snapshot -i         # Interactive elements only (recommended)
cloak-agent snapshot -c         # Compact mode (less tokens)
cloak-agent snapshot -d 3       # Limit depth
cloak-agent snapshot -s "#main" # Scope to CSS selector
```

Output looks like:

```
- button "Submit" [ref=e1]
- textbox "Email" [ref=e2]
- link "Sign up" [ref=e3]

Stats: 3 refs, 89 chars, ~23 tokens
```

Use the refs in subsequent commands: `cloak-agent click @e1`

### Interactions

```bash
cloak-agent click @e1                # Click
cloak-agent fill @e2 "text"          # Clear field and type
cloak-agent type @e2 "text"          # Type without clearing
cloak-agent press Enter              # Press key
cloak-agent hover @e1                # Hover
cloak-agent check @e1                # Check checkbox
cloak-agent select @e1 "value"       # Select dropdown
cloak-agent scroll down 500          # Scroll
```

### Getting info

```bash
cloak-agent get title               # Page title
cloak-agent get url                 # Current URL
cloak-agent get text @e1            # Element text
cloak-agent get html @e1            # Element HTML
cloak-agent get value @e2           # Input value
cloak-agent is visible @e1          # Check visibility
```

### Screenshots

```bash
cloak-agent screenshot              # To stdout (base64)
cloak-agent screenshot output.png   # To file
cloak-agent screenshot --full       # Full page
```

### Stealth (cloak-agent only)

These don't exist in other browser CLIs:

```bash
cloak-agent stealth status               # Run 30 bot detection tests
cloak-agent fingerprint rotate           # New browser identity
cloak-agent fingerprint rotate --seed 42 # Deterministic fingerprint
cloak-agent profile create shopping      # Persistent browser profile
cloak-agent profile list                 # List profiles
```

### Updates

```bash
cloak-agent upgrade                 # Self-update to latest release
cloak-agent version                 # Print current version
```

The CLI checks for updates in the background (once every 24 hours) and prints a notice after your command finishes. No interruptions, no delays — if an update is available, you'll see a one-liner suggesting `cloak-agent upgrade`.

### Tabs, cookies, network, and more

```bash
# Tabs
cloak-agent tab                     # List tabs
cloak-agent tab new https://x.com   # New tab
cloak-agent tab 2                   # Switch to tab

# Cookies
cloak-agent cookies                 # Get all
cloak-agent cookies set name value  # Set cookie
cloak-agent cookies clear           # Clear all

# Wait
cloak-agent wait @e1                # Wait for element
cloak-agent wait 2000               # Wait N ms
cloak-agent wait --load networkidle # Wait for network idle

# State
cloak-agent state save auth.json    # Save session
cloak-agent state load auth.json    # Restore session

# Network
cloak-agent network requests        # View tracked requests
cloak-agent network route <url> --abort  # Block requests
```

## For AI agents

cloak-agent follows the principles from [Rewrite Your CLI for AI Agents](https://justin.poehnelt.com/posts/rewrite-your-cli-for-ai-agents/).

### Raw JSON mode

Agents can skip the human-friendly flags and send the exact payload:

```bash
cloak-agent --json '{"action":"navigate","url":"https://example.com","waitUntil":"networkidle"}'
```

### Schema introspection

Agents can discover what commands exist and what parameters they take:

```bash
cloak-agent schema              # List all commands
cloak-agent schema navigate     # Show navigate's parameters
```

### Dry run

Validate a command without executing it:

```bash
cloak-agent --dry-run open https://example.com
# Would navigate to https://example.com
```

### Context window discipline

Limit response size with `--fields`:

```bash
cloak-agent --fields "url,title" get title
```

Every snapshot response includes a token count estimate so agents can decide whether to request more detail or scope down.

### Input hardening

The daemon validates all input from agents:
- Rejects path traversals (`../../.ssh/id_rsa`)
- Strips control characters
- Validates ref format before lookup

### Global flags

| Flag | What it does |
|------|-------------|
| `--session <name>` | Named session (parallel browsers) |
| `--json` | JSON output mode |
| `--json '{...}'` | Raw JSON payload |
| `--timeout <ms>` | Command timeout |
| `--headed` | Show browser window |
| `--dry-run` | Validate without executing |
| `--fields <list>` | Limit response fields |

## Examples

### Login and save session

```bash
cloak-agent open https://app.com/login
cloak-agent snapshot -i
cloak-agent fill @e1 "user@email.com"
cloak-agent fill @e2 "password"
cloak-agent click @e3
cloak-agent wait --url "/dashboard"
cloak-agent state save auth.json

# Later — restore the session
cloak-agent state load auth.json
cloak-agent open https://app.com/dashboard
```

### Stealth browsing with fingerprint rotation

```bash
cloak-agent open https://bot.sannysoft.com
cloak-agent stealth status
# Pass all tests

cloak-agent fingerprint rotate
# Browser restarts with new identity
cloak-agent stealth status
# Still passing
```

### Parallel sessions

```bash
cloak-agent --session shop1 open https://store-a.com
cloak-agent --session shop2 open https://store-b.com
cloak-agent --session shop1 snapshot -i
cloak-agent --session shop2 snapshot -i
```

## Architecture

```
Go CLI binary          Unix socket          Node.js daemon
┌──────────┐    JSON    ┌──────────────────────┐
│ parse    │ ────────> │ Zod validate          │
│ args     │           │ CloakBrowser launch() │
│ format   │ <──────── │ ARIA snapshots        │
│ output   │    JSON    │ 70+ actions           │
└──────────┘           └──────────────────────┘
```

More detail: [docs/architecture.md](docs/architecture.md)

## Comparison

| Feature | agent-browser | cloak-agent |
|---------|:---:|:---:|
| Stealth Chromium (source patches) | no | yes |
| Fingerprint randomization | no | yes |
| GPU/platform spoofing | no | yes |
| Bot detection check | no | yes |
| Fingerprint rotation | no | yes |
| Persistent profiles | no | yes |
| Raw JSON payload mode | no | yes |
| Schema introspection | no | yes |
| Dry-run validation | no | yes |
| Input hardening | no | yes |
| ARIA snapshots + @refs | yes | yes |
| Daemon architecture | yes | yes |
| AI-friendly errors | yes | yes |

## Docs

- [Command reference](docs/commands.md) — every command with examples
- [Architecture](docs/architecture.md) — how the CLI and daemon communicate
- [Stealth guide](docs/stealth.md) — fingerprints, profiles, detection evasion

## Running tests

```bash
make test
```

This runs both Go tests (52 tests) and daemon TypeScript tests (69 tests, including integration tests with real browser).

## Project structure

```
cloak-agent/
├── main.go              # CLI entrypoint
├── cmd/                 # Go CLI (parser, client, output, root)
├── daemon/              # Node.js daemon
│   └── src/             # TypeScript source
│       ├── daemon.ts    # Socket server
│       ├── browser.ts   # CloakBrowser + Playwright wrapper
│       ├── snapshot.ts  # ARIA snapshot engine
│       ├── actions.ts   # 70+ command handlers
│       ├── protocol.ts  # Zod schemas
│       ├── stealth.ts   # Fingerprint/profile management
│       └── errors.ts    # AI-friendly error messages
├── skills/              # AI agent skill files
├── scripts/             # Build and install scripts
└── Makefile
```

## Credits

Made by [Ashraf](https://ashrafali.net).

Built on [CloakBrowser](https://github.com/CloakHQ/CloakBrowser) (stealth Chromium) and inspired by [agent-browser](https://github.com/vercel-labs/agent-browser) (Vercel).

Design principles from [Rewrite Your CLI for AI Agents](https://justin.poehnelt.com/posts/rewrite-your-cli-for-ai-agents/) and [Ship Types, Not Docs](https://shiptypes.com).

## License

MIT
