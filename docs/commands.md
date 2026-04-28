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

Launch accepts agent-friendly runtime flags like `--profile`, `--proxy`, `--timezone`, `--locale`, `--viewport`, `--geoip`, `--humanize`, `--human-preset`, `--human-config`, `--fingerprint-seed`, `--platform`, `--gpu-vendor`, `--gpu-renderer`, `--user-agent`, `--executable-path`, `--storage-state`, `--ignore-https-errors`, `--context-options`, and repeatable `--arg`.

## Snapshots

The core tool for agents. Returns the page's accessibility tree with element refs.

```bash
cloak-agent snapshot                # Full accessibility tree
cloak-agent snapshot -i             # Interactive elements only (recommended)
cloak-agent snapshot -c             # Compact output (fewer tokens)
cloak-agent snapshot -d 3           # Limit depth to 3
cloak-agent snapshot -s "#main"     # Scope to CSS selector
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
cloak-agent tab 2                      # Switch to tab 2
cloak-agent tab close                  # Close current tab
```

## Dialogs

```bash
cloak-agent dialog accept              # Accept alert/confirm/prompt
cloak-agent dialog accept "text"       # Accept prompt with text
cloak-agent dialog dismiss             # Dismiss dialog
```

## JavaScript

```bash
cloak-agent eval "document.title"      # Run JavaScript, return result
```

## State management

```bash
cloak-agent state save auth.json       # Save cookies, localStorage, sessionStorage
cloak-agent state load auth.json       # Load saved state (set before navigating)
```

## Tracing and recording

```bash
cloak-agent trace start                # Start recording trace
cloak-agent trace stop trace.zip       # Stop and save trace
cloak-agent record start demo.webm     # Start video recording
cloak-agent record stop                # Stop recording
```

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
cloak-agent upgrade                    # Download latest version and refresh runtime
cloak-agent version                    # Print current version
```

Update checks run in the background (once per 24 hours) and print a notice after your command finishes. No startup delay, no interruptions.

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
