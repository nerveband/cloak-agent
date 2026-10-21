# Upstream comparison: 2026-10-21

cloak-agent 0.6.0 compares agent-browser v0.37.1 through v0.39.1 and
updates the CloakBrowser wrapper from 0.5.10 to 0.6.3. This is a release
comparison, not a claim of complete API parity or a port of upstream main.

## Changes carried into this release

- Snapshot deltas use a full baseline, unchanged revision, or a line splice.
  History is isolated by active frame and capture options. `--full` refreshes it.
- Snapshot refs follow the same DOM element across subsequent snapshots.
  Detached or replaced elements cannot inherit an old ref. Navigation, tab
  activation, and frame selection still invalidate the current ref set.
- Selector-scoped snapshots honor their selector, include an interactive selected
  root and open shadow DOM, and resolve refs inside the selected frame.
- Conditional PNG screenshots compare decoded pixels. History belongs to the tab
  and selector/full-page mode. Unchanged captures omit the image and leave files
  untouched. The threshold is a fraction of changed pixels, not an image-quality
  or per-channel tolerance.
- Lost, malformed, or mismatched responses after a mutating request produce
  `outcome_unknown` and `retryable: false`. The CLI never resends that action.
  Read-only requests do not receive an ambiguous-mutation error.
- Snapshot and screenshot options reject unknown arguments and invalid values.
  Schema metadata conservatively marks semantic commands and JavaScript/CDP
  escape hatches as mutating. File outputs and profiler operations are also
  mutating. Browser-version mismatches never recommend automatic retries.

## Existing behavior retained

CloakBrowser already supplies opt-in humanized input through `launch --humanize`,
with presets and configuration. Its new wrapper remains responsible for Chromium
startup, browser downloads, license validation, proxy behavior, and release-channel
selection. The pinned wrapper is distinct from the browser build selected at launch.
A licensed stable runtime can differ from the keyless runtime and between platforms.

Recording already holds the latest frame between repaints with monotonic timing.
The daemon accepts independent connections while asynchronous commands run, so it
has no agent-browser state mutex to remove. Existing WebMCP list/call commands
remain capability-detected and bounded.

## Upstream features not added

Obscura, auto-connect to externally running Chrome, auth vault login, chat mode,
Kernel, Browser Use Cloud, and eve are outside this CLI's CloakBrowser engine and
local-session model. Their changes do not introduce corresponding features here.
Recording cursor overlays/contact sheets, per-action human input overrides, and
automatic WebMCP catalog summaries remain absent. Session-level humanization and
explicit WebMCP discovery remain the supported interfaces.

The upstream CA feature is not equivalent to this project's Linux browser CA
trust. Go HTTPS clients use the system trust store, but `--ca-cert` still applies
only to the browser launch and is not a new cross-platform updater option.
No certificate-verification bypass was added.

## CLI review

The review used the Command Line Interface Guidelines and the repository's
behavioral audit. Checks cover structured errors on stderr, empty stdout on
validation failure, nonzero exit codes, JSON input and output, preserved request
IDs, dry-run behavior, schema discovery, and argument validation. This release
adds no interactive prompts. The audit is a set of behavioral checks, not a
numerical compliance score or a claim that every command was exhaustively audited.

## Sources

- [agent-browser releases](https://github.com/vercel-labs/agent-browser/releases)
- [agent-browser v0.39.1](https://github.com/vercel-labs/agent-browser/releases/tag/v0.39.1)
- [CloakBrowser](https://github.com/CloakHQ/CloakBrowser)
- [Command Line Interface Guidelines](https://clig.dev/)

Native verification, archive checks, CI status, publication, and device deployment
are recorded with the release evidence. A cross-build does not prove native
Windows or Linux execution.
