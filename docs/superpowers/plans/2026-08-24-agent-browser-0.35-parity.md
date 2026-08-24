# agent-browser 0.35 Parity Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bring cloak-agent forward from its agent-browser 0.33.2 parity baseline to every 0.34.0/0.35.0 feature that applies to cloak-agent’s locally launched CloakBrowser architecture, then ship cloak-agent 0.4.0.

**Architecture:** Add private Linux CA trust as a launch concern: validate and fingerprint PEM/DER input, create an isolated NSS database, launch an ephemeral persistent CloakBrowser context against that database, and delete it on close. Keep named profiles incompatible with private CA trust because mixing their NSS state would persist trust. Record the 0.34 tab-pinning and provider-guide items as non-applicable: cloak-agent neither attaches named sessions to shared external Chrome nor ships agent-browser’s provider/MCP layer.

**Tech Stack:** Go CLI and tests; Node.js 20+, TypeScript, Zod, Vitest, Playwright Core, CloakBrowser; `certutil` from `libnss3-tools`; GoReleaser v2; GitHub Releases.

## Global Constraints

- Preserve untracked root `package.json`, `package-lock.json`, and `video.mp4`.
- Keep private CA trust Linux-only and fail closed on other operating systems.
- Never replace CA trust with `--ignore-https-errors`; hostname, validity, and unrelated-authority verification must remain active.
- Never persist an imported CA in a named profile.
- Keep Linux as the native execution baseline; Darwin and Windows are cross-build verification only.
- Release archives must include the compiled daemon, locked manifests, installer scripts, and current upstream-sync record.

---

### Task 1: CLI and protocol contract

**Files:**
- Modify: `cmd/parser.go`
- Modify: `cmd/parser_launch.go`
- Modify: `cmd/parser_test.go`
- Modify: `cmd/root.go`
- Modify: `cmd/root_test.go`
- Modify: `daemon/src/protocol.ts`
- Modify: `daemon/src/launch-options.ts`
- Test: `daemon/tests/protocol.test.ts`

**Interfaces:**
- Consumes: existing `launch` command and `GlobalFlags` parsing.
- Produces: launch fields `caCert?: string` and `clearCaCert?: boolean`; flags `--ca-cert`, `--no-ca-cert`; environment defaults `CLOAK_AGENT_CA_CERT`, `CLOAK_AGENT_CLEAR_CA_CERT`.

- [ ] Add parser tests proving `--ca-cert PATH`, `--no-ca-cert`, environment fallback, explicit clear precedence, and the conflicts with `--profile` and `--ignore-https-errors`.
- [ ] Run `go test ./cmd/... -run 'CA|Launch' -v` and confirm the new tests fail before implementation.
- [ ] Add the two launch fields to Zod and `buildLaunchOptions`, then implement CLI/environment propagation and actionable conflict errors.
- [ ] Run the focused Go and Vitest protocol tests and confirm they pass.

### Task 2: Isolated NSS trust store

**Files:**
- Create: `daemon/src/ca-trust.ts`
- Modify: `daemon/src/browser.ts`
- Test: `daemon/tests/ca-trust.test.ts`
- Modify: `daemon/tests/browser.test.ts`

**Interfaces:**
- Consumes: `BrowserLaunchOptions.caCert` and the `certutil` executable.
- Produces: `prepareCATrust(path): Promise<{ userDataDir: string; fingerprint: string; cleanup(): Promise<void> }>` and browser lifecycle cleanup.

- [ ] Add tests for PEM bundles, DER certificates, malformed/empty files, missing `certutil`, private directory modes, certificate-content fingerprints, and cleanup.
- [ ] Run `npm --prefix daemon test -- ca-trust.test.ts` and confirm it fails before implementation.
- [ ] Implement bounded certificate loading, SHA-256 content identity, `mkdtemp` mode tightening, `certutil -N`, one `certutil -A` per certificate, stderr-safe errors, and recursive cleanup.
- [ ] Launch CA-enabled sessions through `launchPersistentContext` using only the ephemeral NSS directory; reject named profiles and `ignoreHTTPSErrors`; clean the directory on close and failed launch.
- [ ] Run the CA trust and browser lifecycle tests and confirm they pass.

### Task 3: Runtime HTTPS proof

**Files:**
- Create: `daemon/tests/fixtures/ca/` test-generated at runtime only; do not commit keys.
- Test: `daemon/tests/integration.test.ts`

**Interfaces:**
- Consumes: OpenSSL-generated temporary root/server certificates and the built daemon/browser runtime.
- Produces: behavioral proof that the trusted CA loads a valid-hostname HTTPS page while an unrelated CA remains rejected.

- [ ] Add an integration harness that creates certificates in a temporary directory and starts a loopback HTTPS server.
- [ ] Confirm a normal launch rejects the private root and `ignoreHTTPSErrors` is not used.
- [ ] Confirm `--ca-cert` trusts the matching root for the valid hostname and still rejects a server signed by an unrelated root.
- [ ] Confirm `--no-ca-cert` returns the session to default trust on relaunch.

### Task 4: Documentation and parity record

**Files:**
- Modify: `README.md`
- Modify: `docs/commands.md`
- Create: `docs/upstream-sync-2026-08-24.md`
- Modify: `.goreleaser.yaml`
- Modify: `CHANGELOG.md`
- Modify: `cmd/root.go`

**Interfaces:**
- Consumes: agent-browser main commit `8ec2bb0` and release notes for 0.34.0/0.35.0.
- Produces: cloak-agent 0.4.0 release contract and operator instructions.

- [ ] Document `--ca-cert`, `--no-ca-cert`, both environment variables, Linux `certutil` prerequisites, conflict rules, and security properties.
- [ ] Record why persistent external-CDP tab binding and provider-specific deployment skills are not applicable to cloak-agent.
- [ ] Update the archived parity record included by GoReleaser and add the 0.4.0 changelog entry.
- [ ] Set `cmd.Version` to `0.4.0` and verify `go run . version` prints `cloak-agent v0.4.0`.

### Task 5: Clean build and package verification

**Files:**
- Verify: repository and `.goreleaser.yaml`

**Interfaces:**
- Consumes: completed source, locked npm dependencies, GoReleaser configuration.
- Produces: native Linux binary plus Darwin/Linux/Windows release archives and checksums.

- [ ] Run `npm --prefix daemon ci`, `make clean`, `make build`, and `make test` on DevBox.
- [ ] Run the CLI against the built daemon for version, schema, launch, CA trust, navigation, snapshot, and close smoke paths.
- [ ] Run `goreleaser release --snapshot --clean` and inspect every archive for the executable, compiled daemon, locked manifests, installers, and current docs.
- [ ] Extract the Linux amd64 archive into a clean temporary directory and repeat the behavioral smoke test from the packaged files.

### Task 6: Publish and deploy

**Files:**
- Publish: Git commit, `main`, tag `v0.4.0`, GitHub release assets.
- Deploy: DevBox, wavedepth, SHAMS M1, and MacBook using each host’s matching archive.

**Interfaces:**
- Consumes: verified source commit and GoReleaser release artifacts.
- Produces: public GitHub release `v0.4.0` and verified `cloak-agent v0.4.0` installations.

- [ ] Commit only tracked implementation files plus this plan; leave the three unrelated root files untracked.
- [ ] Push `main`, create signed/annotated tag `v0.4.0`, run the non-snapshot GoReleaser release, and verify the GitHub release and checksums directly with `gh`.
- [ ] Verify hostname, user, OS, and architecture on each target before choosing an artifact.
- [ ] Install with the packaged installer without exposing services publicly or copying secrets; do not weaken SSH or Tailscale controls.
- [ ] Run `cloak-agent version` and a local launch/navigation/snapshot/close smoke test on every supported target; report platform-specific blockers as blockers rather than native-pass claims.
