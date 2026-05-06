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
