---
name: stealth
description: Stealth features exclusive to cloak-agent
invariants:
  - Run stealth status after launch to verify detection evasion
  - Use fingerprint rotate when switching tasks or identities
  - Use persistent profiles for sites that track returning visitors
  - Enable geoip with proxies for timezone/locale consistency
---

# Stealth Commands (cloak-agent exclusive)

## Quick Reference
- `cloak-agent stealth status` -- run bot detection tests
- `cloak-agent fingerprint rotate` -- new browser fingerprint
- `cloak-agent fingerprint rotate --seed 42` -- deterministic fingerprint
- `cloak-agent profile create <name>` -- create persistent profile
- `cloak-agent profile list` -- list profiles

## How Stealth Works
cloak-agent uses CloakBrowser -- a Chromium binary patched at the C++ source level.
Fingerprints are baked into the browser binary, not injected via JavaScript.
Detection sites see a real browser because it IS a real browser.

## Stealth Status
Run after launch to verify:
```bash
cloak-agent open https://bot.sannysoft.com
cloak-agent stealth status
# Returns pass/fail for each detection test
```

## Fingerprint Rotation
Creates a new browser identity (GPU, screen, hardware profile):
```bash
cloak-agent fingerprint rotate
# Browser restarts with new fingerprint
cloak-agent stealth status  # verify
```

## Persistent Profiles
Avoid incognito detection (-10% penalty on BrowserScan):
```bash
cloak-agent profile create shopping
# Launch with profile:
# Set CLOAK_AGENT_PROFILE=shopping before running commands
```

## Proxy + GeoIP
Auto-detect timezone and locale from proxy IP:
```bash
# Set proxy via env or launch options
# CLOAK_AGENT_PROXY=http://user:pass@proxy:8080
# GeoIP auto-configures timezone and locale
```
