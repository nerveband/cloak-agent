---
name: cloak-agent
description: Stealth browser automation CLI for AI agents. Use when automating browsers, scraping sites with bot detection, filling forms, navigating pages, or any task requiring undetectable browser control. Replaces agent-browser with stealth Chromium that passes all bot detection tests.
trigger: Use when the user asks to automate a browser, scrape a website, interact with a web page, fill forms, click buttons, take screenshots, or any browser-based task. Especially use when stealth/anti-detection is needed (Cloudflare, reCAPTCHA, bot detection).
invariants:
  - Always snapshot after navigation to get fresh refs
  - Use snapshot -i (interactive) by default to minimize tokens
  - Refs (@e1, @e2) are only valid until the next navigation
  - Use fill instead of type for input fields (fill clears first)
  - Run stealth status after launch to verify detection evasion
  - Use fingerprint rotate when switching tasks or identities
  - Use --dry-run for destructive actions when unsure
---

# Browser Automation with cloak-agent

Stealth browser automation CLI built for AI agents. Uses CloakBrowser's patched Chromium (C++ source-level patches, not JavaScript injection) so bot detection sees a real browser — because it is one.

## Installation

### From source

```bash
git clone https://github.com/nerveband/cloak-agent.git
cd cloak-agent
cd daemon && npm install && cd ..
make build
make install
```

Installs to `~/.cloak-agent/` and symlinks to `/usr/local/bin/`. The stealth Chromium binary (~200MB) downloads automatically on first run.

### Prerequisites

- Go 1.22+
- Node.js 20+
- npm

## Quick start

```bash
cloak-agent open https://example.com
cloak-agent snapshot -i
# - link "More information..." [ref=e1]

cloak-agent click @e1
cloak-agent stealth status
# All 30 detection tests passed
```

## Core workflow

1. **Navigate:** `cloak-agent open <url>`
2. **Snapshot:** `cloak-agent snapshot -i` (returns elements with refs like `@e1`, `@e2`)
3. **Interact** using refs from the snapshot
4. **Re-snapshot** after navigation or significant DOM changes
5. **Verify stealth:** `cloak-agent stealth status` after launch

## Architecture

Two processes, one job:

```
Go CLI (sub-ms)  <--Unix socket-->  Node.js daemon (long-running)
  parse args                          CloakBrowser via Playwright
  send JSON                           ARIA snapshots + 70+ actions
  format output                       Zod validation
```

The daemon starts automatically on first command and stays alive between commands.

## Commands

### Navigation

```bash
cloak-agent open <url>                    # Navigate to URL
cloak-agent open <url> --wait networkidle # Wait for network idle
cloak-agent back                          # Go back
cloak-agent forward                       # Go forward
cloak-agent reload                        # Reload page
cloak-agent close                         # Close browser and daemon
```

### Snapshot (page analysis)

```bash
cloak-agent snapshot            # Full accessibility tree
cloak-agent snapshot -i         # Interactive elements only (recommended)
cloak-agent snapshot -c         # Compact output (fewer tokens)
cloak-agent snapshot -d 3       # Limit depth to 3
cloak-agent snapshot -s "#main" # Scope to CSS selector
```

Output format:

```
- button "Submit" [ref=e1]
- textbox "Email" [ref=e2]
- link "Sign up" [ref=e3]

Stats: 3 refs, 89 chars, ~23 tokens
```

### Interactions (use @refs from snapshot)

```bash
cloak-agent click @e1               # Click
cloak-agent dblclick @e1            # Double-click
cloak-agent focus @e1               # Focus element
cloak-agent fill @e2 "text"         # Clear and type
cloak-agent type @e2 "text"         # Type without clearing (keystroke-by-keystroke)
cloak-agent press Enter             # Press key
cloak-agent press Control+a         # Key combination
cloak-agent keydown Shift           # Hold key down
cloak-agent keyup Shift             # Release key
cloak-agent hover @e1               # Hover
cloak-agent check @e1               # Check checkbox
cloak-agent uncheck @e1             # Uncheck checkbox
cloak-agent select @e1 "value"      # Select dropdown
cloak-agent scroll down 500         # Scroll page
cloak-agent scrollintoview @e1      # Scroll element into view
cloak-agent drag @e1 @e2            # Drag and drop
cloak-agent upload @e1 file.pdf     # Upload files
```

### Get information

```bash
cloak-agent get title               # Page title
cloak-agent get url                 # Current URL
cloak-agent get text @e1            # Element text
cloak-agent get html @e1            # Element innerHTML
cloak-agent get value @e2           # Input value
cloak-agent get attr @e1 href       # Get attribute
cloak-agent get count ".item"       # Count matching elements
cloak-agent get box @e1             # Get bounding box
```

### Check state

```bash
cloak-agent is visible @e1          # Check if visible
cloak-agent is enabled @e1          # Check if enabled
cloak-agent is checked @e1          # Check if checked
```

### Screenshots & PDF

```bash
cloak-agent screenshot              # To stdout (base64)
cloak-agent screenshot output.png   # Save to file
cloak-agent screenshot --full       # Full page
cloak-agent pdf output.pdf          # Save as PDF
```

### Wait

```bash
cloak-agent wait @e1                # Wait for element
cloak-agent wait 2000               # Wait milliseconds
cloak-agent wait --text "Success"   # Wait for text
cloak-agent wait --url "/dashboard" # Wait for URL pattern
cloak-agent wait --load networkidle # Wait for network idle
cloak-agent wait --fn "window.ready" # Wait for JS condition
```

### Stealth (cloak-agent exclusive)

These don't exist in other browser CLIs:

```bash
cloak-agent stealth status                # Run 30 bot detection tests
cloak-agent fingerprint rotate            # New browser identity (random seed)
cloak-agent fingerprint rotate --seed 42  # Deterministic fingerprint
cloak-agent profile create shopping       # Persistent browser profile
cloak-agent profile list                  # List profiles
```

**What gets patched at the C++ level:**
- `navigator.webdriver` removed
- GPU renderer/vendor randomized
- Screen dimensions from fingerprint seed
- Hardware concurrency randomized
- Device memory randomized
- Platform string matches configured OS
- Canvas, WebGL, audio, fonts noise
- CDP automation signals removed

**Stealth tips:**
- Run `stealth status` after launch to verify all 30 tests pass
- Use `fingerprint rotate` when switching tasks or identities
- Use `fingerprint rotate --seed 42` for deterministic identity (same seed = same fingerprint)
- Use persistent profiles for sites that track returning visitors (avoids incognito detection)
- Use proxies with geoip for timezone/locale consistency

### Semantic locators (alternative to refs)

```bash
cloak-agent find role button click --name "Submit"
cloak-agent find text "Sign In" click
cloak-agent find label "Email" fill "user@test.com"
cloak-agent find first ".item" click
cloak-agent find nth 2 "a" text
```

### Browser settings

```bash
cloak-agent set viewport 1920 1080      # Set viewport size
cloak-agent set device "iPhone 14"      # Emulate device
cloak-agent set geo 37.7749 -122.4194   # Set geolocation
cloak-agent set offline on              # Toggle offline mode
cloak-agent set headers '{"X-Key":"v"}' # Extra HTTP headers
cloak-agent set credentials user pass   # HTTP basic auth
cloak-agent set media dark              # Emulate color scheme
```

### Cookies & Storage

```bash
cloak-agent cookies                     # Get all cookies
cloak-agent cookies set name value      # Set cookie
cloak-agent cookies clear               # Clear cookies
cloak-agent storage local               # Get all localStorage
cloak-agent storage local key           # Get specific key
cloak-agent storage local set k v       # Set value
cloak-agent storage local clear         # Clear all
```

### Tabs

```bash
cloak-agent tab                         # List tabs
cloak-agent tab new [url]               # New tab
cloak-agent tab 2                       # Switch to tab
cloak-agent tab close                   # Close tab
```

### Network

```bash
cloak-agent network requests               # View tracked requests
cloak-agent network requests --filter api   # Filter requests
cloak-agent network route <url>             # Intercept requests
cloak-agent network route <url> --abort     # Block requests
cloak-agent network route <url> --body '{}' # Mock response
cloak-agent network unroute [url]           # Remove routes
```

### Dialogs

```bash
cloak-agent dialog accept [text]    # Accept dialog
cloak-agent dialog dismiss          # Dismiss dialog
```

### JavaScript

```bash
cloak-agent eval "document.title"   # Run JavaScript
```

### State management

```bash
cloak-agent state save auth.json    # Save session state
cloak-agent state load auth.json    # Load saved state
```

### Debugging

```bash
cloak-agent open example.com --headed   # Show browser window
cloak-agent console                     # View console messages
cloak-agent console --clear             # Clear console
cloak-agent errors                      # View page errors
cloak-agent errors --clear              # Clear errors
cloak-agent highlight @e1               # Highlight element
cloak-agent trace start                 # Start recording trace
cloak-agent trace stop trace.zip        # Stop and save trace
cloak-agent record start ./demo.webm    # Record video
cloak-agent record stop                 # Stop recording
```

### Schema introspection (for AI agents)

```bash
cloak-agent schema                  # List all commands
cloak-agent schema navigate         # Show navigate's parameters
```

### Self-update

```bash
cloak-agent upgrade                 # Self-update to latest release
cloak-agent version                 # Print current version
```

## Sessions (parallel browsers)

```bash
cloak-agent --session shop1 open https://store-a.com
cloak-agent --session shop2 open https://store-b.com
cloak-agent --session shop1 snapshot -i
cloak-agent --session shop2 snapshot -i
cloak-agent session list
```

## Global flags

| Flag | What it does |
|------|-------------|
| `--session <name>` | Named session (parallel browsers) |
| `--json` | JSON output mode |
| `--json '{...}'` | Raw JSON payload |
| `--timeout <ms>` | Command timeout |
| `--headed` | Show browser window |
| `--dry-run` | Validate without executing |
| `--fields <list>` | Limit response fields |

## JSON output (for parsing)

```bash
cloak-agent snapshot -i --json
cloak-agent get text @e1 --json
```

## Raw JSON mode (for agents)

```bash
cloak-agent --json '{"action":"navigate","url":"https://example.com","waitUntil":"networkidle"}'
```

## Environment variables

| Variable | Description |
|----------|-------------|
| `CLOAK_AGENT_SESSION` | Default session name |
| `CLOAK_AGENT_SOCKET_DIR` | Custom socket directory |
| `CLOAK_AGENT_DAEMON_DIR` | Custom daemon directory |
| `CLOAK_AGENT_HEADED` | Show browser window |
| `CLOAK_AGENT_PROXY` | Proxy server URL |
| `CLOAK_AGENT_PROXY_BYPASS` | Proxy bypass list |
| `CLOAK_AGENT_PROFILE` | Persistent profile name |
| `CLOAK_AGENT_ARGS` | Extra Chromium args (comma-separated) |
| `CLOAK_AGENT_USER_AGENT` | Custom user agent |
| `CLOAK_AGENT_STATE` | Path to storage state JSON |
| `CLOAK_AGENT_IGNORE_HTTPS_ERRORS` | Ignore HTTPS errors |
| `CLOAK_AGENT_EXECUTABLE_PATH` | Custom Chromium binary |

## Example: Form submission

```bash
cloak-agent open https://example.com/form
cloak-agent snapshot -i
# Output: textbox "Email" [ref=e1], textbox "Password" [ref=e2], button "Submit" [ref=e3]

cloak-agent fill @e1 "user@example.com"
cloak-agent fill @e2 "password123"
cloak-agent click @e3
cloak-agent wait --load networkidle
cloak-agent snapshot -i
```

## Example: Login with saved state

```bash
cloak-agent open https://app.com/login
cloak-agent snapshot -i
cloak-agent fill @e1 "user@email.com"
cloak-agent fill @e2 "password"
cloak-agent click @e3
cloak-agent wait --url "/dashboard"
cloak-agent state save auth.json

# Later — restore session
cloak-agent state load auth.json
cloak-agent open https://app.com/dashboard
```

## Example: Stealth browsing with fingerprint rotation

```bash
cloak-agent open https://bot.sannysoft.com
cloak-agent stealth status
# All 30 tests passed

cloak-agent fingerprint rotate
# Browser restarts with new identity
cloak-agent stealth status
# Still passing
```

## Example: Persistent profile for returning visitor

```bash
cloak-agent profile create shopping
# Set CLOAK_AGENT_PROFILE=shopping before commands
CLOAK_AGENT_PROFILE=shopping cloak-agent open https://store.com
# Cookies, localStorage persist across sessions — avoids incognito detection
```

## Example: Proxy with auto geoip

```bash
CLOAK_AGENT_PROXY=http://user:pass@us-proxy:8080 cloak-agent open https://example.com
# Timezone and locale auto-detected from proxy IP
cloak-agent stealth status
```

## Key differences from agent-browser

| Feature | agent-browser | cloak-agent |
|---------|:---:|:---:|
| Stealth Chromium (C++ patches) | no | **yes** |
| Fingerprint randomization | no | **yes** |
| GPU/platform spoofing | no | **yes** |
| Bot detection check | no | **yes** |
| Fingerprint rotation | no | **yes** |
| Persistent profiles | no | **yes** |
| Raw JSON payload mode | no | **yes** |
| Schema introspection | no | **yes** |
| Dry-run validation | no | **yes** |
| Input hardening | no | **yes** |
| ARIA snapshots + @refs | yes | **yes** |
| All standard browser commands | yes | **yes** |

## Troubleshooting

- **Element not found:** Re-snapshot to get fresh refs. Refs change after navigation.
- **Page not loaded:** Add `cloak-agent wait --load networkidle` after navigation.
- **Blocked by overlay:** Dismiss modals/cookie banners first, then retry.
- **Click fails:** Try `scrollintoview @e1` before clicking. Use `--headed` to debug visually.
- **Detection failing:** Run `stealth status` to check. Try `fingerprint rotate` for a fresh identity.
- **Daemon not starting:** Check Node.js 20+ is installed. Check `~/.cloak-agent/` for socket files.

## Notes

- Refs are stable per page load but change on navigation — always re-snapshot.
- Use `fill` instead of `type` for inputs (fill clears existing text first).
- The daemon auto-starts on first command and stays alive between commands.
- Stealth Chromium binary downloads automatically on first run (~200MB).
- Every snapshot includes token count estimates so agents can decide on detail level.

## Reporting issues

- GitHub: https://github.com/nerveband/cloak-agent/issues
