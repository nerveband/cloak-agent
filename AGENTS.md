# cloak-agent Agent Guide

Use `cloak-agent` as a non-interactive, structured browser automation CLI.

## Default Workflow

1. Launch or navigate:
   `cloak-agent open https://example.com`
2. Snapshot before interacting:
   `cloak-agent snapshot -i`
3. Use returned refs such as `@e1`.
4. Re-snapshot after navigation, form submission, reloads, or dynamic page changes.
5. Prefer JSON for automation:
   `cloak-agent --output json <command>`

When stdout is piped, `cloak-agent` defaults to JSON unless `--output human` is set.

## Guardrails

- Use `--dry-run` before mutating or destructive commands when uncertain.
- Use `--fields`, `--limit`, `--id-only`, or `--count` to reduce output.
- Use `fill` for inputs unless you specifically need keystroke-by-keystroke `type`.
- Treat refs as stale after page changes.
- Use `--timeout <ms>` for slow sites.
- Use `doctor` when install, daemon, or runtime state is unclear.

## Machine Interfaces

- Command schema: `cloak-agent schema`
- Single command schema: `cloak-agent schema launch`
- JSON input: `cloak-agent --input json --output json < payload.json`
- JSON file input: `cloak-agent --input-file payload.json --output json`
- Raw payload shorthand: `cloak-agent --output json '{"action":"url"}'`

## Errors

Structured JSON errors include `code`, `message`, `hint`, and `retryable`.

Exit codes:

- `0`: success
- `64`: validation or input error
- `69`: daemon, browser, or network error
- `70`: timeout
- `1`: internal or command failure

## Common Mistakes

- Clicking `@e1` from an old snapshot after navigation.
- Dumping full snapshots when `snapshot -i -c --max-depth 3` is enough.
- Forgetting `--output json` in scripts when stdout is not piped.
- Using `type` for inputs that should be replaced with `fill`.
- Ignoring daemon state instead of running `cloak-agent doctor`.

## DevBox Linux builds and cleanup

- On `devbox.wavedepth.com`, verify `pwd`, `hostname`, and `id -un` before building or moving artifacts. The repository path is `/home/nerveband/src/tools/cloak-agent` and the account is `nerveband`.
- Treat `./cloak-agent`, `./dist/`, `./daemon/dist/`, and `./daemon/node_modules/` as generated and recoverable. `make clean` removes the local binary and compiled daemon output. Remove `daemon/node_modules/` only when reinstalling it immediately with `cd daemon && npm ci`.
- Dependency directories copied from macOS are not valid Linux installs. Native modules under `daemon/node_modules/` must match the current host. Use `file` on native `.node` bindings when diagnosing a copied install.
- Build and test Linux from a clean dependency install with `cd daemon && npm ci`, then run `make build` and `make test`. Confirm the resulting `./cloak-agent` with `file ./cloak-agent` and run `./cloak-agent version`.
- Cross-platform Go release artifacts use `.goreleaser.yaml`. Run `goreleaser release --snapshot --clean` for local verification; it builds Darwin, Linux, and Windows archives without publishing. Validate archive checksums and inspect extracted binaries with `file`. A successful cross-build does not prove execution on macOS or Windows.
- Publishing is tag-driven through the established GoReleaser configuration. Do not create or push a release tag, run a non-snapshot release, or claim deployment without explicit release authorization and direct GitHub evidence.
- Preserve untracked files until their ownership and purpose are clear. In particular, root-level package manifests and media files are not part of the daemon build merely because similarly named tracked files exist under `daemon/`.
