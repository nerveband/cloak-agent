# Upstream and agent-browser parity — 2026-09-14

Primary comparison points:

- agent-browser `v0.37.1` (release commit `ba8f048`; prior cloak-agent parity baseline: `0.35.0` at `8ec2bb0`)
- agent-browser `v0.37.0` (`471ab38`), `v0.36.0` (`eb05921`), `v0.35.2` (`118af8e`), `v0.35.1` (`fbd046c`)
- CloakBrowser `v0.5.10` (locked wrapper; prior lock: `0.5.8`)

## CloakBrowser lock

- npm wrapper: `cloakbrowser` `0.5.10` (`js/src/config.ts` still ships
  `CHROMIUM_VERSION = 146.0.7680.177.5` for the keyless free tier).
- verified keyless Linux x64 runtime: Chromium `146.0.7680.177.5` (unchanged).
- Pro Stable runtime for a licensed key: Chromium `151.0.7922.108.3`.
- `playwright-core` `1.62.1` and `ws` `8.21.2` remain locked.

Wrapper behavior that reaches cloak-agent:

- 0.5.10 aborts a launch when a supplied license key is invalid or cannot be
  validated (`js/src/download.ts`) instead of silently using the free binary.
  cloak-agent surfaces the wrapper error unchanged; keyless launches are
  unaffected because no key is present.
- `geoip` resolution failures now abort the launch (`fix(geoip)`), and
  `info` reports session seats as `active/limit` with a real denial reason.
- Humanized selector resolution (`get_by_*`, Shadow DOM ordering, actionability,
  `display: contents`) is fixed inside the wrapper, which cloak-agent already
  uses through `humanize`/`humanPreset` launch options.

## Imported

**Recording rewrite (0.37.0).** agent-browser replaced `Page.captureScreenshot`
polling with `Page.startScreencast`, a 30 fps default, `--fps 1-60`, current-page
capture, ffmpeg preflight, extension validation, and wall-clock frame timing.
cloak-agent's `record` commands were stubs, so this landed as
`daemon/src/recording.ts` with the same contract in the CloakBrowser/Playwright
architecture: its own CDP session, a ticker that holds the last frame through
gaps (bounded backfill), libvpx for `.webm` / libx264 otherwise, fail-fast
validation ahead of browser state, `No frames captured` for an empty take, and a
`restart` that keeps the previous take. `doctor` now reports ffmpeg.

**Session setup inheritance for new tabs (0.37.0).** Tabs opened with `tab new`
or through clicks inherit headers, credentials, user agent, locale, timezone,
geolocation, offline mode, routes, color scheme, and init scripts before their
first navigation. In cloak-agent most of that is context-level and was already
inherited; page-level media emulation was the gap. `set media`/color scheme now
records the state on the manager and replays it on every new page, awaited before
`tab new <url>` navigates.

**Stale-ref invalidation (0.35.1).** agent-browser resets ref numbering across
snapshots and invalidates refs across URL navigation. cloak-agent now clears the
ref map, cached tree, and frame scope when the active page's main frame
navigates, so a post-navigation click reports `Unknown ref` instead of acting on
a re-resolved role/name match. A failed snapshot still preserves the previous
refs because the map is only replaced on success.

**Stream URL tracking (0.35.1).** The viewport stream now reports the streamed
page's main-frame URL: an initial `url` message, one per full-document, History
API, or fragment navigation (deduplicated), and `metadata.url` on frames. The
implementation enables the CDP `Page` domain on the stream session, which raw
Playwright CDP sessions do not enable by default.

**Stream hardening (0.35.2).** agent-browser hardened dashboard provenance
against DNS rebinding, smuggling, and cross-origin access. cloak-agent has no
dashboard and no reverse-proxy mode, so the applicable part is the local
WebSocket surface: the handshake now requires a loopback `Host` header (with the
bound port when present) alongside the existing origin rejection, inbound
payloads are capped at 256 KB, and client input is schema-validated so NaN
coordinates, unknown actions, and oversized keys never reach CDP.

## Not applicable

- **WebMCP (0.36.0, output in 0.37.0).** The API (`navigator.modelContext`) is
  absent from the locked keyless runtime: probe on Chromium `146.0.7680.177.5`
  returned `undefined` for `modelContext`, `tools`, and `getTools`, and
  agent-browser enables the feature with `--enable-features=WebMCPTesting,
  DevToolsWebMCPSupport`, which CloakBrowser's own argument handling replaces
  with its stealth feature set. Implementing tool discovery and invocation
  cannot be verified on any runtime this release supports, so it is deferred
  until the Pro Chromium 151 line becomes the baseline.
- **`--pin-tab` / persistent target binding, and the CDP root-URL query fix
  (0.34.0, 0.35.2).** cloak-agent owns one CloakBrowser context per session and
  never attaches to a shared external Chrome, so there is no external target to
  bind or a `--cdp` URL to normalize.
- **Windows ARM64 launcher selection (0.35.1) and the private-desktop headless
  Chrome isolation (0.37.1).** Windows binaries are cross-built and inspected,
  not run natively, and CloakBrowser's launcher owns process/desktop setup.
- **Dashboard token/IPv4-IPv6 provenance work, MCP tool profiles, the WebMCP
  generation skill, eve integration updates, and the Lightpanda serve-timeout
  removal.** These belong to agent-browser's provider, MCP, and hosted layers,
  or to engines cloak-agent does not use.
- **Rust dependency bumps (`rustls-webpki`, `quinn-proto`, `brace-expansion`)
  and the npm publish approval workflow.** cloak-agent ships Go and Node
  dependencies and publishes through GoReleaser.

## Verification boundary

Linux x64 is the native runtime baseline: `npm test` in `daemon/` (including the
browser integration file) and `go test ./cmd/...` run natively here, and the
recording path was exercised end to end through the CLI with `ffprobe`
confirmation of container, duration, and frame count. Darwin and Windows
archives are cross-built and inspected; they are not native runtime tests.
