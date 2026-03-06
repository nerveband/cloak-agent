# Stealth guide

cloak-agent uses [CloakBrowser](https://github.com/CloakHQ/CloakBrowser), a Chromium build with fingerprint patches applied at the C++ source level. This is different from tools like puppeteer-stealth or playwright-stealth that inject JavaScript — those patches can be detected by examining the page's JavaScript environment. CloakBrowser's patches are compiled into the browser binary itself.

## What gets patched

CloakBrowser modifies Chromium source code to randomize:

- **navigator.webdriver** — removed (normally `true` in automated browsers)
- **GPU renderer/vendor** — configurable (Apple on macOS, NVIDIA on Windows by default)
- **Screen dimensions** — derived from fingerprint seed
- **Hardware concurrency** — randomized
- **Device memory** — randomized
- **Platform string** — matches configured OS
- **Timezone** — configurable, auto-detected from proxy IP with geoip
- **Language/locale** — configurable, auto-detected from proxy IP

All derived from a single seed number, so the same seed produces the same fingerprint.

## Checking detection status

After launching the browser, run:

```bash
cloak-agent stealth status
```

This navigates to detection test sites and reports results. A clean run passes all 30 tests.

## Fingerprint rotation

To get a new identity without manually closing and reopening:

```bash
cloak-agent fingerprint rotate
```

The browser closes, then relaunches with a new random seed. State (cookies, etc.) is not preserved across rotations.

For reproducible fingerprints:

```bash
cloak-agent fingerprint rotate --seed 42
```

The same seed always produces the same fingerprint.

## Persistent profiles

Regular browser automation runs in incognito mode. Some detection services (like BrowserScan) penalize incognito sessions with a -10% score. Persistent profiles avoid this.

```bash
# Create a profile
cloak-agent profile create shopping

# Use it (set before launching)
export CLOAK_AGENT_PROFILE=shopping
cloak-agent open https://store.com

# List profiles
cloak-agent profile list
```

Profiles are stored under `~/.cloak-agent/profiles/<name>/`. They persist cookies, localStorage, and browser history across sessions.

## Proxy with GeoIP

When using a proxy, CloakBrowser can auto-detect the proxy's geographic location and set the timezone and locale to match. This prevents the common detection signal of a US timezone with a European proxy IP.

Set the proxy via environment variable:

```bash
export CLOAK_AGENT_PROXY=http://user:pass@proxy:8080
```

GeoIP detection is enabled at launch. The browser's timezone and locale will match the proxy's exit IP.

## Default fingerprint settings

On macOS (Apple Silicon):
- Platform: macOS
- GPU vendor: Google Inc. (Apple)
- GPU renderer: ANGLE (Apple, ANGLE Metal Renderer: Apple M3)

On Linux/Windows:
- Platform: Windows
- GPU vendor: NVIDIA Corporation
- GPU renderer: NVIDIA GeForce RTX 3070

You can override any of these at launch via environment variables or the raw JSON launch command:

```bash
cloak-agent --json '{"action":"launch","platform":"windows","gpuVendor":"AMD","gpuRenderer":"Radeon RX 7900"}'
```

## What this doesn't do

cloak-agent handles browser fingerprinting. It doesn't solve:

- **CAPTCHAs** — you still need to solve or bypass these separately
- **IP reputation** — use clean residential proxies for that
- **Behavioral detection** — if you click 100 buttons in 1 second, that's suspicious regardless of fingerprint
- **Account-level flags** — if an account is already flagged, a new fingerprint won't help

The stealth layer makes the browser look real. What the agent does with it is up to you.
