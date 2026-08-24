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
- Node.js 20+
- npm

### From source

```bash
git clone https://github.com/nerveband/cloak-agent.git
cd cloak-agent
make build
```

This builds the Go binary (`./cloak-agent`) and compiles the daemon TypeScript. The `install` flow below also installs daemon dependencies and checks the CloakBrowser runtime.

### Install globally

```bash
make install
# or, from a source checkout or an already installed binary layout:
./cloak-agent install
```

Source installs copy the binary and daemon to `~/.cloak-agent/`, install daemon production dependencies, run `cloakbrowser install`, and expose `cloak-agent` through an existing writable PATH directory when possible. If no writable PATH directory exists, the installer links to `~/.local/bin` and prints the PATH line only when needed. Installed-layout installs run the same daemon/bootstrap steps in place.

Release archives bundle the Go CLI with the compiled daemon and its locked npm
manifests. Keep the executable and `daemon/` directory together, then run
`./cloak-agent install` on Linux or macOS. On Windows, extract the complete ZIP
and run:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/install.ps1
```

This installs the daemon dependencies and CloakBrowser runtime in the extracted
layout. Windows CI verifies the native Go build and unit-level daemon behavior.
Browser execution still requires an explicit native Windows runtime test; a Go
cross-build is not a native test.

To remove only the installed executable and daemon while preserving browser
profiles, Tailgate config, and state:

```bash
./scripts/uninstall.sh
```

Use `./scripts/uninstall.sh --purge` only when you explicitly want those user
profiles and config roots removed as well.

Current runtime baseline (locked in `daemon/package-lock.json`):

- `cloakbrowser` `0.5.8` for the patched Chromium runtime and stable/preview release channels.
- `playwright-core` `1.62.1` for browser control.
- `ws` `8.21.2` for the optional CDP-backed local viewport stream server.

## Quick start

```bash
# Navigate to a page
cloak-agent open https://example.com
cloak-agent open                         # Launch on about:blank
cloak-agent read                         # Agent-friendly text from the active page
cloak-agent keyboard type "hello"        # Type into the focused element

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

Interaction patterns: [docs/interaction-playbook.md](docs/interaction-playbook.md). This playbook explicitly credits [browser-harness](https://github.com/browser-use/browser-harness) as inspiration for its screenshot-first, interaction-skills style and raw-CDP fallback mindset.

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

### Per-browser tailgate

Tailgate can route one named browser session through a loopback-only SSH SOCKS tunnel without changing the host route table:

```bash
cloak-agent --session routed --tailgate launch --profile account https://example.com
cloak-agent --session routed tailgate status
cloak-agent --session routed tailgate stop
```

Set up a route on a clean clone without copying a key:

```bash
cloak-agent tailgate setup egress-host --key ~/.ssh/id_tailgate \
  --known-hosts ~/.ssh/known_hosts
# or use a loaded ssh-agent key:
cloak-agent tailgate setup egress-host --agent --known-hosts ~/.ssh/known_hosts
```

`tailgate setup`, `tailgate doctor`, `--direct`, `CLOAK_AGENT_TAILGATE`, and
`CLOAK_AGENT_DIRECT` are portable across Linux/macOS and use XDG config/state
roots. If the existing Browser Harness Tailgate wrapper is present, the
default route can be discovered without redundant config; `tailgate import`
persists it. Configuration, key-only SSH requirements, profile isolation,
recovery, and diagnostics are documented in [docs/tailgate.md](docs/tailgate.md).

### Launch / daemon lifecycle

Use `launch` when an agent wants to establish a session with explicit CloakBrowser options instead of generating ad-hoc Node scripts.

```bash
cloak-agent launch https://example.com \
  --profile shopping \
  --proxy http://proxy:8080 \
  --timezone America/New_York \
  --locale en-US \
  --viewport 1440x900 \
  --user-agent CustomAgent/1.0 \
  --storage-state state.json \
  --geoip \
  --humanize \
  --human-preset careful \
  --context-options '{"permissions":["geolocation"]}' \
  --fingerprint-seed 42 \
  --arg --disable-gpu

# Linux-only private proxy CA trust; requires certutil from libnss3-tools/nss-tools.
cloak-agent launch https://internal.example --proxy http://proxy:8080 \
  --ca-cert /etc/ssl/certs/proxy-ca.pem
cloak-agent launch https://example.com --no-ca-cert

cloak-agent daemon start
cloak-agent daemon status
cloak-agent daemon logs
cloak-agent daemon restart
cloak-agent daemon stop
```
`--ca-cert` accepts a DER certificate or PEM bundle and imports it into a
private, temporary NSS database used only by that browser session. Normal
hostname, expiry, and unrelated-authority checks remain enabled. It cannot be
combined with `--profile` or `--ignore-https-errors`. The equivalent
`CLOAK_AGENT_CA_CERT` and `CLOAK_AGENT_CLEAR_CA_CERT=1` environment variables
are supported. Install `libnss3-tools` on Debian/Ubuntu or `nss-tools` on RPM
Linux before using it.


### Updates

```bash
cloak-agent install                 # Bootstrap local daemon deps/browser runtime
cloak-agent upgrade                 # Self-update, then refresh daemon deps/browser runtime
cloak-agent version                 # Print current version
```

The CLI checks for updates in the background (once every 24 hours) and prints a notice after your command finishes. No interruptions, no delays — if an update is available, you'll see a one-liner suggesting `cloak-agent upgrade`.

### Tabs, cookies, network, and more

```bash
# Tabs
cloak-agent tab                     # List tabs
cloak-agent tab new https://x.com   # New tab
cloak-agent tab new --label docs https://docs.example.com
cloak-agent tab t2                  # Switch by stable tab id
cloak-agent tab docs                # Or switch by label

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

# JavaScript / CDP
cloak-agent eval "document.title"
cloak-agent cdp Runtime.evaluate '{"expression":"document.title","returnByValue":true}'

# Network
cloak-agent network requests        # View tracked requests
cloak-agent network requests --type xhr,fetch --status 2xx
cloak-agent network request r1      # Full request/response metadata
cloak-agent network route <url> --abort  # Block requests
cloak-agent network unroute         # Remove all registered routes
```

Other synchronized Agent Browser ergonomics include `goto`/`navigate` aliases,
`keydown`/`keyup`, `keyboard inserttext`, `get styles`, `wait --state`,
`storage session`, `frame`, `dialog status`, `pushstate`, and
`profiler start|stop`.

CloakBrowser 0.5 launch controls are available through
`launch --release-channel stable|preview`, `--browser-version`, and repeatable
`--extension` flags.

## For AI agents

cloak-agent follows the principles from [Rewrite Your CLI for AI Agents](https://justin.poehnelt.com/posts/rewrite-your-cli-for-ai-agents/).

### Structured JSON I/O

For machine-readable output, use `--output json` (or legacy `--json`). When stdout is piped, cloak-agent defaults to JSON unless `--output human` is set. For machine-readable input, use `--input json` or `--input-file`.

```bash
cloak-agent --output json daemon status
echo '{"action":"navigate","url":"https://example.com"}' | cloak-agent --input json --output json
cloak-agent --input-file payload.json --output json
```

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
cloak-agent schema launch       # Show launch-only fields and option names
```

### Dry run

Validate a command without executing it:

```bash
cloak-agent --dry-run open https://example.com
# Would navigate to https://example.com
```

### Context window discipline

Limit response size with `--fields`, `--limit`, `--id-only`, `--count`, and snapshot depth controls:

```bash
cloak-agent --fields "url,title" get title
cloak-agent --output json --limit 5 network requests
cloak-agent --output json --count profile list
cloak-agent snapshot -i -c --max-depth 3
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
| `--output json` | Stable machine-readable output |
| `--output human` | Force human output even when stdout is piped |
| `--json` | Alias for `--output json`; also works as legacy raw-JSON shorthand when followed by a JSON object |
| `--quiet`, `-q` | Suppress update notices and status noise |
| `--input json` | Read command JSON from stdin |
| `--input-file <path>` | Read command JSON from file |
| `--timeout <ms>` | Command timeout |
| `--headed` | Show browser window |
| `--dry-run` | Validate without executing |
| `--yes`, `-y`, `--force` | Non-interactive confirmation flag for future destructive flows |
| `--fields <list>` | Limit response fields |
| `--limit <n>` | Limit collection output |
| `--id-only` | Return only identifiers where possible |
| `--count` | Return counts for collections where possible |
| `--ca-cert <path>` | Trust a private CA for a Linux browser launch |
| `--no-ca-cert` | Start the new launch without retained private CA trust |

### Structured errors and exit codes

JSON errors include `code`, `message`, `hint`, and `retryable`.

| Code | Meaning |
|------|---------|
| `0` | Success |
| `64` | Validation or input error |
| `69` | Daemon, browser, or network error |
| `70` | Timeout |
| `1` | Internal or command failure |

## Troubleshooting

- `node` or `npm` missing: `cloak-agent install` now fails early with a direct prerequisite message instead of a shell stack trace.
- Daemon startup failure: run `cloak-agent --output json daemon status` and `cloak-agent --output json daemon logs` to inspect the socket path, pid file, and latest log output.
- Install/runtime mismatch: run `cloak-agent --output json doctor`.
- Runtime uncertainty: `doctor` includes daemon runtime proof when available, including active page, tabs, and context count.
- CloakBrowser missing: run `cloak-agent install`; source installs and installed-layout installs both run `npx cloakbrowser install`.
- Working from a source checkout: the repo-built `./cloak-agent` now resolves `daemon/dist/daemon.js` from the checkout itself, so smoke tests and local development use the current code instead of an older installed daemon copy.
- Viewport stream clients can read the local WebSocket port from `cloak-agent --output json doctor` (`streamPort`) or `~/.cloak-agent/<session>.stream`.

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
- [Interaction playbook](docs/interaction-playbook.md) — practical browser workflows, credited to browser-harness as inspiration
- [Architecture](docs/architecture.md) — how the CLI and daemon communicate
- [Stealth guide](docs/stealth.md) — fingerprints, profiles, detection evasion
- [Tailgate routing](docs/tailgate.md) — per-session SSH SOCKS routing

## Running tests

```bash
make test
```

This runs both Go tests and daemon TypeScript tests, including integration tests with a real CloakBrowser launch.

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
