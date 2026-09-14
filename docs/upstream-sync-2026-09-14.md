# Upstream comparison and review corrections: 2026-09-14

Comparison baseline: agent-browser v0.37.1 and CloakBrowser wrapper v0.5.10.
The previous cloak-agent baseline was agent-browser v0.35.0 and wrapper v0.5.8.
This is a comparison of releases, not a claim that every change on upstream main
was ported or that both CLIs expose identical APIs.

## Wrapper and browser versions

- Locked wrapper: CloakBrowser 0.5.10. Playwright Core 1.62.1 and ws 8.21.2 remain pinned.
- Keyless Linux x64: Chromium 146.0.7680.177.5. Keyless macOS: 145.0.7632.109.2.
- At review, upstream advertised licensed stable 151.0.7922.108.6 on Linux/Windows
  and 151.0.7922.108.3 on macOS. These are platform-dependent runtime versions,
  not the npm package version.
- The latest runtime is also available with a free GitHub-sign-in key limited to
  one concurrent session. A key is not necessarily a paid subscription.
- Invalid/unreachable license validation and failed requested geoip resolution now
  fail launch. Humanized selector/actionability fixes come from the pinned wrapper.

Source: [CloakBrowser README](https://github.com/CloakHQ/cloakbrowser#cloakbrowser-pro).

## WebMCP correction

The original claim that keyless Chromium 146 could not support WebMCP was incorrect.
The original CLI probe used an argument form that was silently ignored and did not
establish secure-context support. CloakBrowser's argument builder preserves user
feature overrides; it does not categorically remove the WebMCP flags.

A secure-origin probe on keyless Chromium 146 successfully registered, listed, and
executed a tool through `navigator.modelContextTesting`. The remediation adds opt-in
`launch --webmcp`, frame-scoped `webmcp list` and `webmcp call`, bounded inputs,
outputs, and timeouts, and explicit unsupported errors. It does not implement
agent-browser's separate detached invocation/history interface or claim CDP-domain
compatibility on every browser build. Runtime capability detection is authoritative.

## Ported behavior and corrections

- Recording uses its own CDP screencast and ffmpeg. The remediation fixes exact-rate
  monotonic timing, signal/exit/stdin failures, bounded cleanup, output staging,
  final-file byte counts, and preflight before replacing a running take.
  Excessive backlog fails explicitly rather than silently distorting duration.
- Ref state belongs to a page and snapshot generation. Navigation, activation, and
  close invalidate it; a snapshot completing after the page changes is rejected.
- Media updates merge committed state. Explicit tabs await setup before navigation.
  Popups receive asynchronous replay; their first script is not guaranteed to see
  dynamic overrides. Subsequent CLI actions wait for replay and surface failures.
- The stream tracks full-document, history, and fragment URLs. Capture and input are
  pinned to the same page until reconnection, even if the CLI switches tabs.
- Native-only stream clients must omit Origin. Opaque `null`, file, and HTTP origins
  are rejected. Host remains loopback-only; messages have a 256 KiB limit and typed
  input validation.

## Release review corrections

The v0.5.0 release was published while CI was failing. New recording tests lacked
ffmpeg in CI; macOS/Windows also had pre-existing Linux-only CA-test failures.
Those are now addressed in workflow/test changes. Publication is tag-driven and
requires successful platform, browser-integration, and archive jobs.

The original updater only replaced the executable. The remediation installs a
verified complete bundle into an immutable release directory and atomically selects
it through `current.json`; a stable launcher binds CLI and daemon to the same release.
Existing incompatible daemons are not automatically terminated. The first migration
from the old updater requires a complete release archive.

The SSH ownership diagnosis was also overstated: the agent namespace maps only UID
1000, so unmapped root IDs appear as nobody. Use the private SSH configuration with
`ssh -F` inside that namespace. Do not change host ownership based on that view.

## Verification boundaries

Regression tests cover recorder failures, timing, refs, media merging, stream origin
rejection/input targeting, WebMCP execution, and installation rollback/archive safety.
Native smoke evidence and platform limitations must be reported with each release.
Cross-build success does not establish native macOS or Windows browser behavior.
