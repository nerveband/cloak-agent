# Upstream and agent-browser parity — 2026-08-03

The compatibility review used primary release metadata and changelogs:

- [CloakBrowser releases](https://github.com/CloakHQ/CloakBrowser/releases)
- [CloakBrowser changelog](https://github.com/CloakHQ/CloakBrowser/blob/main/CHANGELOG.md)
- [agent-browser releases](https://github.com/vercel-labs/agent-browser/releases)
- [agent-browser changelog](https://github.com/vercel-labs/agent-browser/blob/main/CHANGELOG.md)

Context7 (`ctx7`) was not installed on DevBox, so the release pages, npm
metadata, checked-in lockfiles, and upstream changelogs were used directly.

## CloakBrowser lock

- cloak-agent release candidate: **0.3.0** (previous published release was
  0.2.0).
- npm wrapper: `cloakbrowser` **0.5.3**, locked in
  `daemon/package-lock.json`. This wrapper fixes proxy-derived identity reuse
  between multiple browser launches, which is required for per-browser route
  isolation.
- Verified free Linux x64 runtime: Chromium **146.0.7680.177.5**. The latest
  Pro Linux runtime is **150.0.7871.114.4** when a valid Pro entitlement is
  present; no entitlement is assumed or bundled.
- `playwright-core` **1.62.1** and `ws` **8.21.2** are locked alongside it.
- Native Linux execution is the release baseline. Darwin/Windows archives are
  cross-built and inspected, not reported as native runtime tests.

## Parity matrix

| Area | agent-browser v0.33.2 | cloak-agent release behavior | Decision |
|---|---|---|---|
| CLI and JSON | Broad CLI plus JSON/MCP interfaces | Go parser, daemon Zod schema, stable response envelope, `schema` introspection | Keep envelope and align parser/schema/help/tests |
| Lifecycle | daemon/session startup, idle shutdown, restore/close recovery | persistent Go/Node daemon, session sockets, daemon start/stop/restart/status/logs, Tailgate recovery | Implement route-aware launch/close/stop and stale-state checks |
| Profiles/state | named profiles, namespaces, storage restore | named persistent profiles, storage state, session+route Tailgate namespaces | Keep physical direct/routed separation; document deliberate profile API differences |
| Navigation/interactions | refs, semantic finders, keyboard/mouse, tabs, dialogs | same core actions plus CloakBrowser-specific launch/fingerprint controls | Maintain command parity where schemas fit; preserve Cloak options |
| Diagnostics | console/errors/network/HAR, axe accessibility, screenshots/diffing | console/errors/network, stealth diagnostics, snapshots, screenshots, schema/doctor | Add no Rust/MCP dependency; keep diagnostics local and structured |
| Streaming | latest-wins frames, max FPS, ack pacing, sequence/epoch timestamps | local CDP WebSocket stream with bounded loopback endpoint | Preserve local stream contract; document missing agent-browser pacing/mobile clients |
| Policies/security | domain allowlists, WebRTC controls, providers/dashboard | strict daemon validation, path checks, CloakBrowser sandbox defaults, Tailgate SSH key-only/strict-host-key tunnel | Deliberate integration-layer difference; no sandbox weakening |
| Updates | npm/binary update flow | checksum-verified cloak-agent updater plus locked daemon/runtime bootstrap | Keep established install/upgrade contract and rollback-safe staged install |

Not imported deliberately: agent-browser's Rust runtime, MCP/providers/dashboard,
mobile clients, HAR body storage, screenshot diffing, and its policy surface.
Those features do not map cleanly onto CloakBrowser's Go+Node architecture and
would broaden the security surface of this release. They remain tracked for a
future parity review.
