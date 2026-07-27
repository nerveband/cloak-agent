# Upstream synchronization — 2026-07-27

This release audits Agent Browser `v0.21.2..v0.33.0` (184 non-merge commits,
187 commits including release/merge history) and updates Cloak Agent from
CloakBrowser `0.3.31` to `0.5.2`.

## Mirrored Agent Browser changes

| Upstream theme | Cloak Agent result |
| --- | --- |
| Browser launch without navigation and navigation aliases | `open` with no URL plus `goto` and `navigate` |
| Keyboard reliability and focused-element input | `keydown`, `keyup`, `key`, `keyboard type`, and `keyboard inserttext` |
| Agent-readable page content | `read [url]` using the rendered DOM |
| Element inspection | `get styles` |
| Wait reliability | `wait <selector> --state attached|detached|visible|hidden`; glob URL patterns remain supported by Playwright |
| Storage parity | Both `storage local` and `storage session` |
| Stable tab targeting | Monotonic `t<N>` ids, optional labels, and legacy numeric-index compatibility |
| Frame targeting | `frame <selector>` and `frame main`; normal selectors honor the active frame |
| Dialog reliability | Alert/beforeunload auto-accept, pending prompt/confirm status, explicit accept/dismiss |
| Network request inspection | Stable request ids, type/method/status filters, request and response headers, post data, and detail lookup |
| SPA navigation | `pushstate <url>` with Next router support and History API fallback |
| Chrome profiling | `profiler start` and `profiler stop [path]` |
| Profile/launch option reliability | Existing relaunch, storage-state, profile, timeout, daemon recovery, and viewport-stream behavior retained and retested |

## CloakBrowser 0.5 compatibility

- Runtime package: `cloakbrowser ^0.5.2`.
- Control package: `playwright-core ^1.62.0`.
- Stable/preview channel selection: `--release-channel`.
- Exact binary pin: `--browser-version`.
- Repeatable extension loading: `--extension`.
- Authenticated proxy and GeoIP fixes are inherited from the new wrapper.
- The dependency override is `tar 7.5.22`; the daemon audit is clean.

## Audited but intentionally not copied

Agent Browser’s provider integrations, dashboard/chat UI, Rust-native daemon,
Safari/iOS drivers, AWS AgentCore support, sandbox packages, Eve extension,
plugin marketplace, React DevTools protocol, MCP server, and its Chrome-for-
Testing installer are separate product/runtime layers. Cloak Agent keeps its
Go CLI + TypeScript daemon + CloakBrowser architecture, so copying those
subsystems would replace rather than synchronize the product.

The upstream `a11y`, HAR-body capture, screenshot-diff, and standalone HTTP
markdown discovery features remain candidates for later focused releases.
This release mirrors the generally applicable browser-control and reliability
surface while preserving CloakBrowser stealth behavior.
