Cloak Agent Agent-Browser Parity Execution Plan

Date: 2026-03-19
Repo: /Users/ashrafali/cloak-agent
Goal: make cloak-agent feel and work like an agent-first browser CLI, minimizing ad-hoc Node script generation and maximizing reliable direct CLI use by LLMs and shell agents.

Primary Objectives

1. Finish the partially implemented local changes safely.
2. Make cloak-agent usable as a first-class persistent browser control CLI.
3. Improve agent DX in line with:
   - agent-browser ergonomic patterns
   - shiptypes / agent-first CLI principles
   - agent-dx-cli-scale priorities
4. Ensure CloakBrowser is auto-installed or checked when missing.
5. Preserve or improve current test coverage.
6. Release, install, and verify a new working local version.

Current Known State

Baseline findings
- Go CLI + Node daemon architecture already exists.
- Baseline before edits:
  - `go test ./...` passed
  - `cd daemon && npm test` passed
  - `make build` passed
- Repo already includes:
  - socket-based daemon model
  - raw JSON command mode
  - schema introspection
  - snapshot/ref workflow
  - stealth integration via CloakBrowser

Reference audit conclusions
`vercel-labs/agent-browser` patterns to emulate:
- persistent daemon per session
- shell-native command UX
- simple install/bootstrap story
- strong session ergonomics
- concise human output, structured machine output
- broad but regular action families
- configuration/env layering
- explicit browser/runtime install workflows
- agent-friendly defaults

Local changes already partially applied
These may exist in working tree and must be reviewed before continuing:
- Go client:
  - response compatibility for `ok` vs `success`
  - daemon log file support
  - daemon stop support
- parser:
  - `launch` command support with more options
  - `daemon start|stop|restart|status|logs`
  - `--output json`
  - `--input json`
  - `--input-file`
- root command:
  - improved install flow
  - special handling for daemon commands
  - stdin/file JSON input support
- daemon:
  - cloakbrowser dependency bumped to `^0.3.18`
  - BrowserManager launch option expansion
  - action forwarding expansion

Known failing test
- `TestParseArgs_LaunchWithOptions`
- Current issue:
  - test expects `viewport` as `map[string]interface{}`
  - parser emits `map[string]int`
- This is minor and should be fixed first.

---

Execution Strategy

Follow strict order:
1. Inspect and stabilize local diff
2. Fix tests
3. Complete missing feature work
4. Re-run full test/build suite repeatedly until green
5. Run manual smoke tests
6. Review UX against objectives
7. Commit/push/release/install
8. Final verification

Do not release anything before:
- Go tests pass
- daemon tests pass
- build passes
- smoke tests pass

---

Phase 1 — Inspect Current Working Tree

Tasks
1. Review local git state
   - `git status --short`
   - `git diff --stat`
   - `git diff`
2. Identify all partial edits already made.
3. Confirm no unrelated files were accidentally touched.
4. Summarize modifications by subsystem:
   - Go CLI
   - parser
   - daemon protocol/actions/browser
   - docs/tests

Success criteria
- Clear understanding of current in-progress changes.
- No accidental unrelated edits remain unnoticed.

---

Phase 2 — Fix Immediate Breakage

Task 2.1 — Fix failing Go test
Current failure:
- `TestParseArgs_LaunchWithOptions`

Likely fixes:
- preferred: adjust test to accept the actual viewport representation intentionally emitted by parser
- alternative: normalize parser output to `map[string]interface{}`
- choose whichever is more consistent with the rest of parser output and daemon marshaling
Task 2.2 — Re-run Go tests
Run:
- `go test ./cmd/...`
- then `go test ./...`

Success criteria
- all Go tests green before further work

---

Phase 3 — Finish Core Agent-First CLI Improvements

3A — Machine-readable I/O contract

Goals
- Make structured usage explicit and predictable.
- Remove ambiguity around `--json`.

Tasks
1. Confirm or finish support for:
   - `--output json`
   - `--input json`
   - `--input-file <path>`
2. Preserve backward compatibility with existing `--json` where possible.
3. Ensure raw JSON input works via:
   - literal payload arg
   - stdin
   - file input
4. Make help text/documentation explicit about each path.
5. Confirm formatter uses a single consistent response model.

Desired behavior
- Human mode:
  - concise text output
- Machine mode:
  - consistent JSON envelope
- Input mode:
  - explicit raw payload entry points

Success criteria
- no ambiguity for LLMs/scripts about how to send or receive JSON

---

3B — Daemon lifecycle UX

Goals
Allow LLMs and users to manage the persistent daemon directly without ad-hoc scripts.

Tasks
1. Finish and verify:
   - `cloak-agent daemon start`
   - `cloak-agent daemon stop`
   - `cloak-agent daemon restart`
   - `cloak-agent daemon status`
   - `cloak-agent daemon logs`
2. Ensure daemon startup:
   - verifies `node` exists
   - provides good error messages
   - writes logs to a stable per-session location
3. Ensure `status` prints:
   - session
   - running/stopped
   - socket path
   - pid file
   - log file
4. Ensure stop cleans up:
   - pid file
   - socket file when appropriate

Success criteria
- daemon lifecycle can be controlled entirely from CLI
- failures are debuggable by reading `daemon logs`

---

3C — Install/bootstrap flow

Goals
User should not have to manually figure out daemon and CloakBrowser installation.

Tasks
1. Finish `cloak-agent install` behavior so it performs:
   - daemon dependency install
   - daemon build
   - CloakBrowser binary install/check
2. Ensure good messages when:
   - npm is missing
   - node is missing
   - cloakbrowser install fails
3. Confirm local install script behavior remains compatible with repo workflow.
4. Decide whether install should:
   - always run `npx cloakbrowser install`
   - or first detect whether CloakBrowser binary already exists
5. Prefer idempotent behavior.

Success criteria
- one command is enough to bootstrap a usable local environment

---

3D — CloakBrowser option compatibility

Goals
Expose as much of CloakBrowser’s capability surface as practical through cloak-agent.

High-priority options
- headless / headed
- profile
- proxy
- timezone
- locale
- geoip
- fingerprint seed
- user agent
- viewport
- extra browser args
- executable path
- storage state
- ignore HTTPS errors

Tasks
1. Audit parser → command map
2. Audit protocol schema
3. Audit actions forwarding
4. Audit BrowserManager launch signature
5. Ensure types line up end-to-end
6. Add tests for supported launch options

Success criteria
- `launch` can express a broad CloakBrowser-compatible option set
- options are actually forwarded, not just parsed

---

3E — Parser / protocol parity cleanup

Goals
Eliminate drift between Go CLI parser and daemon Zod schemas.

Known examples to inspect
- `select`
- `scroll`
- `get attr`
- `set device`
- `set offline`
- `dialog`
- `network route`
- any command where Go field names don’t match daemon schema fields

Tasks
1. Compare every parser-emitted action shape with protocol schema.
2. Fix mismatches.
3. Add or improve tests to cover CLI -> JSON mapping.
4. Prefer one canonical field naming scheme.

Success criteria
- every parsed CLI command validates cleanly against daemon schemas
- no silent shape mismatches remain

---

3F — Better schema / introspection UX

Goals
Make runtime discoverability more useful for agents.

Tasks
1. Review current `schema` output.
2. Improve command metadata if feasible:
   - required fields
   - optional fields
   - enums
   - examples
   - aliases
3. Ensure `schema` remains lightweight enough for agent use.
4. If not doing full JSON Schema now, at least improve consistency and completeness.

Success criteria
- `cloak-agent schema`
- `cloak-agent schema <command>`
provide genuinely useful machine guidance

---

3G — Docs and help polish

Goals
Ensure the LLM can succeed from CLI help/docs without external improvisation.

Tasks
1. Update README examples
2. Update command reference
3. Update install docs
4. Add daemon-control examples
5. Add JSON input/output examples
6. Add launch option examples
7. Add troubleshooting notes:
   - node missing
   - daemon startup failure
   - cloakbrowser missing
   - how to inspect logs

Success criteria
- README and help reflect the actual CLI behavior
- docs teach the intended agent workflow directly

---

Phase 4 — Test Coverage Expansion

4A — Go CLI tests
Add/adjust tests for:
- launch with extended options
- daemon command parsing
- `--output json`
- `--input json`
- `--input-file`
- response compatibility with `ok`
- install path behavior if practical
- daemon status/log formatting if practical

4B — Daemon tests
Add/adjust tests for:
- launch option forwarding
- protocol validation for expanded launch command
- any fixed parser/protocol parity mismatches
- compatibility with cloakbrowser version bump

4C — Build validation
Run:
- `go test ./...`
- `cd daemon && npm test`
- `cd daemon && npm run build`
- `cd .. && make build`

Success criteria
- all automated validation green

---

Phase 5 — Manual Smoke Testing

Run these manually after tests pass.

Bootstrap
- `cloak-agent install`

Daemon
- `cloak-agent daemon start`
- `cloak-agent daemon status`
- `cloak-agent daemon logs`

Browser launch
- `cloak-agent launch https://example.com --timezone America/New_York --locale en-US --viewport 1440x900`
- `cloak-agent get title`
- `cloak-agent snapshot -i`

JSON I/O
- `echo '{"action":"title","id":"x1"}' | cloak-agent --input json --output json`
- `cloak-agent --output json get url`
- `cloak-agent --input-file payload.json --output json`

Session behavior
- `cloak-agent --session test daemon status`
- `cloak-agent --session test open https://example.com`
- `cloak-agent --session test snapshot -i`

CloakBrowser compatibility
Test representative launch flags:
- `--profile`
- `--proxy`
- `--timezone`
- `--locale`
- `--geoip`
- `--fingerprint-seed`
- `--arg`

Success criteria
- normal CLI workflows no longer require writing one-off Node files
- daemon control is first-class
- JSON workflows are predictable

---

Phase 6 — Code Review Against Objectives

Before release, check the final implementation against:

Agent-browser parity
- persistent daemon workflow
- session ergonomics
- launch ergonomics
- install/bootstrap flow
- command usability
- concise outputs

Agent DX / shiptypes principles
- machine-readable output
- explicit structured input
- introspection
- context-discipline
- hardened input
- safety rails where relevant
- packaged knowledge in docs/skills

Success criteria
- the CLI is meaningfully more reliable and easier for LLMs to use directly

---

Phase 7 — Release Workflow

Tasks
1. Review final diff
   - `git status`
   - `git diff --stat`
2. Commit with a clean message, e.g.:
   - `feat: add daemon control and agent-first launch/install UX`
3. Push to GitHub
4. Create a new release
   - use existing goreleaser/release flow if configured
5. Build/install new release locally
6. Verify installed binary reports expected version

Suggested release note themes
- daemon lifecycle commands
- explicit JSON input/output support
- improved install/bootstrap flow
- improved CloakBrowser compatibility
- improved agent-first UX

Success criteria
- code merged/pushed
- release created
- local installed version matches release

Phase 8 — Local Post-Release Verification

After install:
1. `which cloak-agent`
2. `cloak-agent version`
3. `cloak-agent daemon start`
4. `cloak-agent daemon status`
5. `cloak-agent open https://example.com`
6. `cloak-agent snapshot -i`
7. `cloak-agent close`

Success criteria
- released build works on the current machine end-to-end

---

Risks / Watchouts

1. Partial local edits already exist
- inspect carefully before finalizing

2. Parser/protocol drift
- easy to miss unless explicitly checked

3. CloakBrowser API compatibility
- confirm current 0.3.18 signatures actually match forwarding code

4. JSON backward compatibility
- preserve older `--json` behavior where feasible

5. Install flow assumptions
- don’t assume cwd incorrectly if `cloak-agent install` is run outside repo
- if needed, scope install behavior clearly to source-repo installs versus packaged installs

---

Definition of Done

The work is done when all of the following are true:

- Go tests pass
- daemon tests pass
- daemon build passes
- make build passes
- `cloak-agent install` works
- `cloak-agent daemon start|stop|restart|status|logs` work
- `launch` accepts and forwards expanded CloakBrowser options
- JSON input/output modes are explicit and usable
- no ad-hoc Node file creation is needed for standard browser control tasks
- docs/help are updated
- changes are committed, pushed, released, and installed locally
- released binary passes post-release smoke tests
