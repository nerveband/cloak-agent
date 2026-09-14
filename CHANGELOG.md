# Changelog

## 0.5.1 (2026-09-14)

- Install and upgrade complete checksum-verified bundles into immutable release
  directories. Activate a matching executable/daemon pair through one pointer;
  add equal-version repair and rollback without terminating existing sessions.
- Bind daemon lookup to the executable rather than the working directory, verify
  component versions, and reject incompatible active daemons with recovery guidance.
- Make recording output transactional, handle encoder death and stdin failures,
  bound teardown, preserve active takes on invalid restart, use exact-rate monotonic
  timing, and report actual output-file bytes.
- Invalidate refs across page transitions and merge dynamic media settings.
  Explicit tabs await setup; popup first-script media inheritance is not guaranteed.
- Keep stream input pinned to the pictured page, reject all supplied Origins
  including null/file origins, and serialize stream lifecycle operations.
- Add opt-in capability-detected WebMCP list/call with frame selection and bounds.
  Correct the prior claim that WebMCP requires a licensed Chromium 151 runtime.
- Require Node.js 22+ for cloak-agent. Fix ffmpeg CI prerequisites and platform
  CA tests, replace the wording-based CLI score with isolated behavioral checks,
  and gate release publication on CI. Snapshot manifests match the built version.
- Correct the prior SSH ownership diagnosis: an unmapped agent namespace can show
  root-owned host files as nobody; that is not evidence of host corruption.

The v0.5.0 entries below describe the original release intent. Recording failure
handling, timing, popup guarantees, and updater completeness were overstated there
and are corrected by this remediation.

## 0.5.0 — 2026-09-14

- Replace the `record` stubs with real video recording. `record start <path>
  [--fps N]`, `record restart`, and `record stop` capture the active page through
  a dedicated CDP screencast piped into `ffmpeg` at 1-60 fps (default 30),
  holding frames so a static page still produces a correctly timed file.
  `.webm` encodes with libvpx and any other extension with libx264. Missing
  ffmpeg, an extensionless path, or an out-of-range rate fails before any
  browser or file state changes; an empty take reports `No frames captured`
  instead of leaving a broken file; `record restart` keeps the previous take.
- New tabs inherit the session's setup before their first navigation, including
  page-level media emulation that previously applied only to the active page.
- Refs are invalidated when the active page navigates (full document, History
  API, or fragment), so a click after navigation reports a stale ref instead of
  binding to whatever now matches the old role and name.
- The viewport stream emits `url` messages and stamps `metadata.url` on frames
  for the streamed page's main-frame navigation. The stream handshake now
  requires a loopback `Host` header in addition to the existing origin checks,
  closes the inbound payload at 256 KB, and schema-validates client input.
- `doctor` reports ffmpeg availability for recording.
- Update the locked CloakBrowser wrapper from 0.5.8 to 0.5.10 (humanized
  selector fixes, seat reporting in `info`, license and geoip launch-abort
  behavior). The keyless Linux x64 runtime remains Chromium 146.0.7680.177.5;
  a license key selects the Chromium 151 Pro build.

## 0.4.3 — 2026-08-24

- Scope update-check caches to the installed version so an upgrade never
  advertises an older cached release as newer.

## 0.4.2 — 2026-08-24

- Update the locked CloakBrowser wrapper from 0.5.3 to 0.5.8. The latest free
  Linux runtime remains Chromium 146.0.7680.177.5.

## 0.4.1 — 2026-08-24

- Make the Unix installer consume prebuilt release archives directly instead
  of requiring the source-only `scripts/build.sh`.

## 0.4.0 — 2026-08-24

- Add Linux private proxy CA trust through `--ca-cert`, isolated temporary NSS
  databases, PEM bundle and DER validation, and explicit `--no-ca-cert`
  clearing without disabling ordinary TLS verification.
- Sync the compatibility review through agent-browser 0.35.0. External Chrome
  tab pinning and provider-specific deployment skills remain intentionally
  outside cloak-agent's local CloakBrowser architecture.

## 0.3.1 — 2026-08-03

- Fix parsing of the documented `tailgate import --route NAME` form.

## 0.3.0 — 2026-08-03

- Add native per-browser Tailgate routing through a loopback-only SSH dynamic
  SOCKS tunnel, with direct mode, named routes, setup/import/doctor/status/stop,
  strict key-only SSH, recovery state, and route/profile isolation.
- Add XDG-aware fresh-install setup, ssh-agent support, Browser Harness
  compatibility discovery/import, atomic staged installation, and a preserving
  uninstall path.
- Lock CloakBrowser 0.5.3, Playwright Core 1.62.1, and ws 8.21.2; document
  current CloakBrowser/runtime and agent-browser parity decisions.
- Harden structured lifecycle, schema/help/docs alignment, stale-state recovery,
  local SOCKS/control endpoint validation, atomic loopback-port reservations,
  and direct/routed profile recovery across daemon restarts.
