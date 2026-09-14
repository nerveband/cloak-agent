# Release remediation implementation plan

## Scope and boundaries

Repair the v0.5.0 review findings without modifying unrelated local policy, manifests, or media. Do not publish or deploy a replacement release during implementation. Do not change host ownership based on an unmapped user-namespace view. Preserve browser profiles, credentials, active user sessions, and rollback installations.

## Runtime

- Recording: validate replacements before stopping, stage output beside its destination, retain existing output on failure, handle encoder spawn/stdin/exit errors, bound preflight and teardown, and require exit code zero. Compute frame slots from a monotonic clock and exact fps; fail explicitly rather than silently distort recordings when buffering limits are exceeded. Report final file bytes.
- CLI: reject zero/negative/out-of-range fps and extra stop arguments; reject unknown launch options instead of silently dropping flags.
- Page state: associate snapshots with their page and generation, invalidate on navigation/activation/close, and merge media updates only after successful application. Playwright has no public dynamic context-media setter: explicit tabs await setup before navigation; popup setup is awaited before subsequent commands, without claiming first-script inheritance.
- Streaming: reject supplied browser origins including opaque/null origins; retain loopback Host validation and input schemas. Capture and input must target the same page. Bound CDP cleanup and eliminate leaked input sessions.
- WebMCP: opt-in launch flags, capability discovery on secure frames, frame-scoped list/call, bounded execution/results, and explicit unsupported errors. No Chromium-major or license assumption.

## Complete installation

- Download the platform release and checksums, verify before extraction, reject unsafe archive paths and links, and validate the executable/daemon version pair.
- Stage dependencies with npm ci and bootstrap the browser before activation. Install immutable version directories and atomically replace a small current-version pointer. Keep old versions for rollback and active processes. Serialize installs with a lock.
- Use the same installation engine for source/archive installs and upgrades. Resolve both executable and daemon from one immutable version. Refuse incompatible active daemons instead of terminating user sessions automatically.
- Check supported Node runtimes separately for cloak-agent and agent-browser. Do not perform a root-owned Node upgrade.

## CI, audit, and release process

- Install ffmpeg for real recording jobs; isolate pure tests from external executables. Fix platform-specific CA tests to test supported and unsupported contracts.
- Replace wording/default-count audit assertions with behavior checks, use unique temporary state, and clean up only audit-owned sessions.
- Build releases from clean detached checkouts of the intended commit, without skip-validate. Gate publication on successful platform checks and archive verification.
- Correct SSH namespace, runtime licensing, WebMCP, timing, popup, and update documentation. Runtime Chromium versions are platform-specific and separate from wrapper versions.

## Acceptance

- Regression scenarios from the review fail before the fix and pass afterward: encoder death, invalid restart, exact-rate scheduling, wrong-tab refs, merged media settings, null Origin, and invalid CLI fps.
- Upgrade tests prove checksum/path rejection, bootstrap failure preservation, version consistency, rollback, and reinstall repair without touching unrelated state.
- Native Linux smoke exercises actual recording, streaming, WebMCP, and install paths. Isolated native Mac smoke uses temporary installations without replacing deployed tools. Windows compilation and platform CI are reported separately from native runtime proof.
- Final report identifies every completed correction and any remaining host/account authorization prerequisite. No claims based only on package version strings.

## SSH diagnosis

The agent's `/proc/self/uid_map` and `gid_map` map only ID 1000. Host root IDs are unmapped and appear as 65534 (`nobody:nogroup`), including `/`, `/etc`, and `/usr/bin/ssh`. This is not proof of damaged host file ownership. Use `ssh -F /home/nerveband/.ssh/config` inside this namespace to retain the user's private host aliases without loading the incompatible system include. Do not change root ownership or weaken SSH permission checks. An ordinary host terminal is the place to verify actual root ownership if a failure also occurs outside the namespace.

## User-approved Node runtime correction

The user selected user-default Node 24. Official Node 24.21.0 archives were checksum
verified and installed under each user's `~/.local/opt`; `~/.local/bin/node` selects
them on DevBox and both Macs. Root-owned Node installations and running services were
not changed. Each Mac's previous Node link is retained inside the new runtime as
`node-before-upgrade`. DevBox previously had no user Node link; removing the new link
restores `/usr/bin/node`. The separate npm and Homebrew package owners remain unchanged.

## Clean release procedure

Do not use `--skip=validate`, remove unrelated files, or stash another task's work.
After the intended changes are committed with authorization, create a detached
worktree at that commit and run validation there. The CI tag-publication job uses a
clean checkout and requires successful platform tests, Linux browser integration,
and release archive checks. Do not push a tag until publication is explicitly approved.
GoReleaser generates archive-only package/lock manifests matching its release or
snapshot version without changing source manifests. Native platform proof remains
separate from cross-compilation.

## Verification completed

- TypeScript build passed. The full Linux suite passed 110 tests with one
  platform-specific skip. The final explicit-empty-Origin change also passed all
  nine stream-server tests.
- Go command/updater tests passed. The isolated CLI audit passed six behavioral
  checks. Malformed input and dry-run no longer depend on documentation wording.
- A real v0.5.0 temporary daemon rejected the new CLI handshake while remaining
  alive. Cleanup stopped only that test session.
- Complete temporary installs succeeded on DevBox and both Macs. Each exercised
  ref invalidation, merged media, pinned stream input, recording restart validation,
  and video decoding. The one-second takes encoded 30 frames into one-second H.264
  files; returned bytes matched file sizes. A recorded Linux frame was inspected.
- Secure-frame WebMCP discovery/execution passed on keyless Linux Chromium 146.
  Keyless macOS Chromium 145 lacked the APIs and returned the expected unsupported
  error. No license was obtained or changed.
- Node 24.21.0 and agent-browser 0.37.1 opened, evaluated, and closed an isolated page
  on both Macs. DevBox's restricted agent namespace blocked the normal Chrome
  sandbox. An about:blank-only smoke passed with a temporary `--no-sandbox` argument;
  no persistent browser flags or system security settings were changed.
- GoReleaser configuration validated. A nonpublishing snapshot built all five
  configured archives. All contained matching package/lock versions and compiled
  daemon files. The Linux archive installed successfully and rolled back to the
  previous managed release. Snapshot mode bypasses release validation by design;
  this does not establish that a tagged release or hosted CI has passed.
- Windows cross-compilation passed. Native Windows execution and the updated
  GitHub Actions workflow remain unexecuted until an authorized CI run.

## Pre-release verification boundary

The source version is 0.5.1, unreleased. No commits, pushes, release tags, publication,
or replacement cloak-agent deployments were performed. Existing v0.5.0 deployments
remain in place. The explicitly approved user-default Node update is the only
cross-device persistent runtime change. Temporary verification installs, scripts,
recordings, and snapshot artifacts were removed after stopping their test sessions
and checking that test daemon PID files were absent. Popup first-script media
inheritance remains outside the guarantee; explicit tabs and subsequent CLI actions
await setup.

The user subsequently authorized release and deployment of v0.5.1. Publication
must pass the tagged CI gates. Deploy verified complete bundles to DevBox and both
Macs, preserve existing sessions, and verify fresh sessions on the selected release.
