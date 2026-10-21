# Command reference

Every command cloak-agent supports, grouped by category. All commands follow the pattern:

```bash
cloak-agent <command> [args...] [flags]
```

Refs like `@e1` come from `snapshot -i`. They change on navigation, so always re-snapshot after page changes.

---

## Navigation

```bash
cloak-agent open <url>                    # Navigate to URL
cloak-agent launch [url] [launch flags]   # Launch browser/session with explicit CloakBrowser options
cloak-agent open <url> --wait networkidle # Wait for network idle after navigation
cloak-agent back                          # Go back
cloak-agent forward                       # Go forward
cloak-agent reload                        # Reload page
cloak-agent close                         # Close browser and stop daemon
```

Launch accepts agent-friendly runtime flags like `--profile`, `--proxy`, `--timezone`, `--locale`, `--viewport`, `--geoip`, `--humanize`, `--human-preset`, `--human-config`, `--fingerprint-seed`, `--platform`, `--gpu-vendor`, `--gpu-renderer`, `--user-agent`, `--executable-path`, `--storage-state`, `--ignore-https-errors`, `--ca-cert`, `--no-ca-cert`, `--context-options`, and repeatable `--arg`.

Global `--tailgate` selects the private `default` SSH route. `--tailgate-route <name>` selects a named route. Use `tailgate status`, `tailgate doctor`, and `tailgate stop` with the same `--session`. See [tailgate.md](tailgate.md).
On Linux, `--ca-cert <path>` imports a DER certificate or PEM bundle into a
private NSS database for the launched browser. It requires `certutil` from
`libnss3-tools` (Debian/Ubuntu) or `nss-tools` (RPM). Hostname, validity, and
unrelated-authority checks stay enabled. Do not combine it with `--profile` or
`--ignore-https-errors`. `CLOAK_AGENT_CA_CERT` supplies the path by environment;
`--no-ca-cert` or `CLOAK_AGENT_CLEAR_CA_CERT=1` explicitly starts without it.


Create a route without editing JSON:

```bash
cloak-agent tailgate setup <ssh-host> --key ~/.ssh/id_tailgate --known-hosts ~/.ssh/known_hosts
cloak-agent tailgate setup <ssh-host> --agent --known-hosts ~/.ssh/known_hosts
cloak-agent tailgate setup <ssh-alias> --ssh-config ~/.ssh/config --route work
cloak-agent tailgate import --route browser-harness
```

`--direct` and `CLOAK_AGENT_DIRECT=1` explicitly select direct egress. The
config path is `$CLOAK_AGENT_TAILGATE_CONFIG`, then
`$XDG_CONFIG_HOME/cloak-agent/tailgate.json`, then
`~/.config/cloak-agent/tailgate.json` (with a legacy fallback when present).

## Snapshots

The core tool for agents. Returns the page's accessibility tree with element refs.

```bash
cloak-agent snapshot                # Full accessibility tree
cloak-agent snapshot -i             # Interactive elements only (recommended)
cloak-agent snapshot -c             # Compact output (fewer tokens)
cloak-agent snapshot -d 3           # Limit depth to 3
cloak-agent snapshot -s "#main"     # Scope to CSS selector
cloak-agent snapshot -i --delta      # Full baseline, then changes only
cloak-agent snapshot -i --delta --full # Refresh the baseline
```

**Output:**

```
- heading "Example Domain" [ref=e1] [level=1]
- button "Submit" [ref=e2]
- textbox "Email" [ref=e3]
- link "Click here" [ref=e4]

Stats: 4 refs, 142 chars, ~36 tokens
```

**Tips:**
- Always use `-i` unless you need the full page structure
- Re-snapshot after any navigation
- Check the token count to gauge page complexity

## Interactions

All interaction commands accept a ref (`@e1`) or CSS selector.

```bash
cloak-agent click @e1                  # Click
cloak-agent dblclick @e1               # Double-click
cloak-agent fill @e2 "text"            # Clear field, then type
cloak-agent type @e2 "text"            # Type without clearing (keystroke by keystroke)
cloak-agent press Enter                # Press key
cloak-agent press Control+a            # Key combination
cloak-agent keydown Shift              # Hold key
cloak-agent keyup Shift                # Release key
cloak-agent hover @e1                  # Hover over element
cloak-agent focus @e1                  # Focus element
cloak-agent check @e1                  # Check checkbox
cloak-agent uncheck @e1                # Uncheck checkbox
cloak-agent select @e1 "value"         # Select dropdown option
cloak-agent upload @e1 file.pdf        # Upload file
cloak-agent drag @e1 @e2               # Drag and drop
cloak-agent scroll down 500            # Scroll down 500px
cloak-agent scroll up 300              # Scroll up 300px
cloak-agent scrollintoview @e1         # Scroll element into view
```

**Use `fill` for inputs, not `type`.** `fill` clears the field first. `type` sends individual keystrokes without clearing — useful for autocomplete fields.

## Getting information

```bash
cloak-agent get title                  # Page title
cloak-agent get url                    # Current URL
cloak-agent get text @e1               # Element text content
cloak-agent get html @e1               # Element innerHTML
cloak-agent get value @e2              # Input field value
cloak-agent get attr @e1 href          # Element attribute
cloak-agent get count ".item"          # Count matching elements
cloak-agent get box @e1                # Element bounding box (x, y, width, height)
```

## Checking state

```bash
cloak-agent is visible @e1             # true/false
cloak-agent is enabled @e1             # true/false
cloak-agent is checked @e1             # true/false
```

## Screenshots and PDF

```bash
cloak-agent screenshot                 # Screenshot to stdout (base64)
cloak-agent screenshot output.png      # Save to file
cloak-agent screenshot --full          # Full page screenshot
cloak-agent screenshot --full page.png # Full page to file
cloak-agent screenshot out.png --if-changed # Save only when pixels change
cloak-agent screenshot out.png --if-changed --threshold 0.01 # Allow 1% changed pixels
cloak-agent pdf output.pdf             # Save page as PDF
```

## Waiting

```bash
cloak-agent wait @e1                   # Wait for element to appear
cloak-agent wait 2000                  # Wait 2 seconds
cloak-agent wait --text "Success"      # Wait for text to appear
cloak-agent wait --url "/dashboard"    # Wait for URL to match pattern
cloak-agent wait --load networkidle    # Wait for network to be idle
cloak-agent wait --fn "window.ready"   # Wait for JS condition
```

## Mouse control

```bash
cloak-agent mouse move 100 200         # Move mouse to coordinates
cloak-agent mouse down left            # Press mouse button
cloak-agent mouse up left              # Release mouse button
cloak-agent mouse wheel 100            # Scroll wheel
```

## Semantic locators

Alternative to refs — find elements by role, text, or label:

```bash
cloak-agent find role button click --name "Submit"
cloak-agent find text "Sign In" click
cloak-agent find label "Email" fill "user@test.com"
```

## Browser settings

```bash
cloak-agent set viewport 1920 1080     # Set viewport size
cloak-agent set device "iPhone 14"     # Emulate device
cloak-agent set geo 37.7749 -122.4194  # Set geolocation
cloak-agent set offline on             # Enable offline mode
cloak-agent set offline off            # Disable offline mode
cloak-agent set headers '{"X-Key":"v"}'# Set HTTP headers
cloak-agent set credentials user pass  # HTTP basic auth
cloak-agent set media dark             # Dark color scheme
cloak-agent set media light            # Light color scheme
```

## Cookies and storage

```bash
cloak-agent cookies                    # Get all cookies
cloak-agent cookies set name value     # Set cookie
cloak-agent cookies clear              # Clear all cookies
cloak-agent storage local              # Get all localStorage
cloak-agent storage local key          # Get specific key
cloak-agent storage local set k v      # Set value
cloak-agent storage local clear        # Clear localStorage
```

## Network

```bash
cloak-agent network requests                   # View tracked requests
cloak-agent network requests --filter api      # Filter by URL pattern
cloak-agent network requests --type xhr,fetch --method POST --status 2xx
cloak-agent network request r1                 # Full request/response metadata
cloak-agent network route <url>                # Intercept and continue matching requests
cloak-agent network route <url> --abort        # Block requests
cloak-agent network route <url> --body '{}'    # Mock response
cloak-agent network route <url> --status 201 --body '{}' # Mock with custom status
cloak-agent network unroute                    # Remove all routes
cloak-agent network unroute <url>              # Remove one route
```

## Tabs

```bash
cloak-agent tab                        # List tabs
cloak-agent tab new                    # New empty tab
cloak-agent tab new https://x.com      # New tab with URL
cloak-agent tab new --label docs https://docs.example.com
cloak-agent tab t2                     # Switch by stable tab id
cloak-agent tab docs                   # Switch by label
cloak-agent tab 2                      # Switch to tab 2
cloak-agent tab close                  # Close current tab
```

Context-level settings are inherited by new pages. Dynamic media updates are merged;
explicit `tab new <url>` waits for replay before navigation. Popup replay completes
before subsequent CLI actions, but not necessarily before the popup's first script.
Navigation, tab activation, and close invalidate refs. Take a fresh snapshot afterward.

## Dialogs

```bash
cloak-agent dialog accept              # Accept alert/confirm/prompt
cloak-agent dialog accept "text"       # Accept prompt with text
cloak-agent dialog dismiss             # Dismiss dialog
cloak-agent dialog status              # Inspect a pending prompt/confirm
```

## JavaScript

```bash
cloak-agent eval "document.title"      # Run JavaScript, return result
cloak-agent cdp Runtime.evaluate '{"expression":"document.title","returnByValue":true}'
```

`cdp` sends a raw Chrome DevTools Protocol method to the active page session. It is a deliberate escape hatch for cases where ordinary refs, selectors, or JavaScript evaluation are not enough. Prefer built-in commands first, keep CDP payloads narrow, and re-snapshot after CDP-driven changes.

## State management

```bash
cloak-agent state save auth.json       # Save cookies, localStorage, sessionStorage
cloak-agent state load auth.json       # Load saved state (set before navigating)
```

## Tracing and recording

```bash
cloak-agent trace start                # Start recording trace
cloak-agent trace stop trace.zip       # Stop and save trace
cloak-agent record start demo.webm     # Record the active page (30 fps default)
cloak-agent record start demo.mp4 --fps 60
cloak-agent record restart take2.webm  # Finalize the take, start the next one
cloak-agent record stop                # Finalize the take
```

`record` captures the active page through a CDP screencast piped into `ffmpeg`.
Requires `ffmpeg` on `PATH` (`doctor` reports it). The output extension selects
the container: `.webm` uses libvpx, anything else uses libx264; an extensionless
path is rejected. `--fps` accepts 1-60. Frames are written at the requested rate
even while the page is static, so the file duration matches the wall-clock take.
`record stop` reports `path`, `frames`, `capturedFrames`, `fps`, `durationMs`,
and final file `bytes`. Output is staged beside the destination and replaced only
after successful finalization. Failed/empty takes preserve existing files.
Invalid restart paths, rates, codecs, or inaccessible directories leave the current
take running. The new path must differ from the current path. Encoder failure,
timeout, and excessive backlog return errors rather than a misleading success.

## WebMCP

```bash
cloak-agent launch https://example.com --webmcp
cloak-agent webmcp list --frame main
cloak-agent webmcp call echo '{"value":"hello"}' --timeout 10000
cloak-agent webmcp list --frame '#embedded-tools'
```

`webmcp_list` and `webmcp_call` are also available through JSON input and schema
introspection. Support is detected in the selected secure frame, not inferred from
the license or Chromium major version. The default timeout is 10 seconds, maximum
60 seconds. Input limit: 1 MiB. Output limit: 2 MiB. Discovery limit: 512 tools.
These are synchronous calls, not agent-browser's detached invocation/history API.
Timeout requests cancellation, without guaranteeing reversal of tool side effects.

## Debugging

```bash
cloak-agent console                    # View console messages
cloak-agent console --clear            # View and clear
cloak-agent errors                     # View page errors
cloak-agent errors --clear             # View and clear
cloak-agent highlight @e1              # Highlight element with red border
```

## Stealth (cloak-agent only)

```bash
cloak-agent stealth status             # Run bot detection tests, return pass/fail
cloak-agent fingerprint rotate         # Close browser, relaunch with new fingerprint
cloak-agent fingerprint rotate --seed 42  # Deterministic fingerprint
cloak-agent profile create myprofile   # Create persistent browser profile
cloak-agent profile list               # List all profiles
```

## Updates

```bash
cloak-agent upgrade                    # Verify and install a complete release
cloak-agent upgrade --repair           # Repair an equal release version
cloak-agent rollback                   # Select the previous managed release
cloak-agent version                   # Print selected release version
./scripts/uninstall.sh                 # Remove binary/daemon, preserve profiles/config
./scripts/uninstall.sh --purge         # Explicitly remove install and user state
```

Source/archive installs and upgrades share one Go installation engine. They stage
the executable, compiled daemon, and locked production dependencies; validate matching
versions; bootstrap the browser; then atomically replace `current.json`.
Old release directories and user profiles/config remain untouched. The stable
bootstrap dispatches to the selected immutable release, independent of the shell cwd.
An incompatible active daemon is rejected without terminating the session.
Use a new session or explicitly restart it. Uninstall refuses active daemons.

`upgrade` verifies GitHub release checksums before safe extraction. `--repair`
reinstalls the same version; it never downgrades to an older upstream release.
The first upgrade from the pre-0.5.1 binary-only updater must use a complete archive.

## Schema introspection (for AI agents)

```bash
cloak-agent schema                     # List all available commands
cloak-agent schema navigate            # Show parameters for 'navigate' command
```

## Sessions

Run multiple browsers in parallel with named sessions:

```bash
cloak-agent --session a open https://site-a.com
cloak-agent --session b open https://site-b.com
cloak-agent session list               # Show active sessions
cloak-agent daemon start               # Start daemon for session
cloak-agent daemon status              # Inspect daemon state
cloak-agent daemon logs                # Read daemon log
cloak-agent daemon restart             # Restart daemon
cloak-agent daemon stop                # Stop daemon
```

## Global flags

These work with any command:

| Flag | Description |
|------|-------------|
| `--session <name>` | Named session (default: "default") |
| `--output json` | Stable machine-readable output |
| `--json` | Alias for `--output json`; also works as legacy raw-JSON shorthand |
| `--input json` | Read command JSON from stdin |
| `--input-file <path>` | Read command JSON from file |
| `--timeout <ms>` | Command timeout in milliseconds |
| `--headed` | Show the browser window |
| `--dry-run` | Validate command without executing |
| `--fields <list>` | Comma-separated list of fields to return |

## Structured JSON examples

```bash
cloak-agent --output json daemon status
echo '{"action":"navigate","url":"https://example.com"}' | cloak-agent --input json --output json
cloak-agent --input-file payload.json --output json
cloak-agent --json '{"action":"snapshot","interactive":true}'   # legacy shorthand
```


### Incremental captures

`snapshot --delta` keeps a baseline for each active frame and option set. The
first response includes `tree`, `revision`, and `baseline: true`. An unchanged
response includes `unchanged: true` and the same revision. Changed responses
include `baseRevision` and `delta: {start, deleteCount, lines}`. Apply that splice
to the previous tree's newline-separated lines. `start` is zero-based. `--full`
returns a fresh full baseline. Navigation, tab changes, and frame changes reset
refs and require a fresh baseline. History retains at most sixteen option sets.

Refs keep their identity while the same DOM element survives. A detached or
replaced element cannot receive actions through its old ref. Each snapshot makes
only the refs in that snapshot available for subsequent actions. Re-snapshot
before acting on a changed page.

`screenshot --if-changed` compares decoded PNG pixels against the last image
returned for that tab and selector/full-page mode. `--threshold` accepts a changed
pixel fraction from zero to one, with zero as the default. Dimensions changing
always produces a new image. An unchanged result returns `changed: false` and
`difference`, with no `path` or `base64`, and does not touch the requested file.
A failed write does not replace the comparison baseline. JPEG conditional
captures are rejected. History retains at most sixteen capture modes and 64 MiB of decoded pixels per
tab. A larger image is returned without retaining a comparison baseline.

If a mutating command's response is lost or invalid, the CLI reports
`outcome_unknown` and `retryable: false`. Inspect the page or session before
retrying because the original action may already have completed. The CLI does
not resend the action.
