# Upstream sync: 2026-08-03

Primary release metadata was checked on 2026-08-03.

- cloak-agent's latest published release is `v0.2.0`; this checkout is post-release main.
- CloakBrowser wrapper `0.5.3` is now locked. It fixes proxy-derived identity reuse between multiple launches in one process, which is required for correct per-browser tailgate isolation.
- The latest keyless CloakBrowser binary is Chromium `146.0.7680.177.5`. Pro Linux has `150.0.7871.114.4`; upstream lists `150.0.7871.114.3` for the corresponding macOS and Windows stable builds. A cross-build is not a native-platform test.
- agent-browser `v0.33.2` adds idle shutdown and stream pacing/input fixes beyond the prior `v0.33.0` audit. cloak-agent retains its Go CLI/Node daemon design. Idle shutdown, restore namespaces, policy controls, HAR bodies, screenshot diffing, MCP/providers/dashboard, and mobile clients remain deliberate differences rather than partial implementations.
- cloak-agent continues to prioritize stable structured JSON, session/profile isolation, lifecycle commands, runtime-backed doctor output, checksum-verified updates, and its CloakBrowser integration.

Primary sources: CloakBrowser's GitHub changelog and releases, its npm package metadata, and agent-browser's `v0.33.2` GitHub release/changelog.
