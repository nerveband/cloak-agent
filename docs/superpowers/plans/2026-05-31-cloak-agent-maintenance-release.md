# Cloak Agent Maintenance Release Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Refresh the CloakBrowser runtime dependency baseline, clear the direct WebSocket advisory, verify the CLI/daemon still work, and publish a patch release.

**Architecture:** Keep the existing Go CLI plus Node daemon split. Limit code changes to version metadata and dependency/docs maintenance so the browser command surface remains stable.

**Tech Stack:** Go, TypeScript, Node.js, CloakBrowser, Playwright, Vitest, GoReleaser.

---

## Chunk 1: Dependency and Version Refresh

### Task 1: Runtime Dependency Baseline

**Files:**
- Modify: `daemon/package.json`
- Modify: `daemon/package-lock.json`
- Modify: `cmd/root.go`

- [x] **Step 1: Confirm current dependency targets**

Run: `npm view cloakbrowser version dist-tags --json`
Expected: `0.3.31` is the npm `latest` tag.

- [x] **Step 2: Update vulnerable direct dependency**

Run: `cd daemon && npm install ws@^8.21.0`
Expected: `npm audit` reports zero vulnerabilities.

- [x] **Step 3: Bump patch version**

Run: `cd daemon && npm version 0.1.5 --no-git-tag-version`
Expected: daemon package metadata reports `0.1.5`.

- [x] **Step 4: Match CLI version**

Set `cmd.Version` to `0.1.5`.

### Task 2: Documentation Refresh

**Files:**
- Modify: `README.md`
- Modify: `docs/architecture.md`
- Modify: `docs/stealth.md`

- [x] **Step 1: Document runtime baseline**

Add the refreshed `cloakbrowser`, `playwright-core`, and `ws` versions.

- [x] **Step 2: Document verification commands**

Add the audit/test/build commands to architecture docs.

## Chunk 2: Verification and Release

### Task 3: Verify Locally

**Files:**
- No source edits expected.

- [x] **Step 1: Run daemon audit**

Run: `cd daemon && npm audit --json`
Expected: zero vulnerabilities.

- [x] **Step 2: Run daemon tests**

Run: `cd daemon && npm test`
Expected: all Vitest tests pass, including the real CloakBrowser integration test.

- [x] **Step 3: Run Go tests**

Run: `go test ./...`
Expected: all packages pass.

- [x] **Step 4: Build release binary**

Run: `make build`
Expected: `./cloak-agent` builds with version `0.1.5`.

- [x] **Step 5: Smoke test built CLI**

Run: `./cloak-agent --version`
Expected: `cloak-agent v0.1.5`.

### Task 4: Publish

**Files:**
- Git metadata only.

- [ ] **Step 1: Commit scoped changes**

Run: `git add ... && git commit -m "chore: refresh cloak browser runtime baseline"`
Expected: commit succeeds.

- [ ] **Step 2: Push main**

Run: `git push origin main`
Expected: remote branch updates.

- [ ] **Step 3: Tag and release**

Run: `git tag v0.1.5 && git push origin v0.1.5 && goreleaser release --clean`
Expected: GitHub release `v0.1.5` is published with artifacts.
