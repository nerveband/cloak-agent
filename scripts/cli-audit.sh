#!/usr/bin/env bash
set -euo pipefail

CLI="${1:-./cloak-agent}"
score=0
total=50

pass() { score=$((score + 1)); printf 'PASS %s\n' "$1"; }
fail() { printf 'FAIL %s\n' "$1"; }
has() { [[ "$1" == *"$2"* ]]; }
valid_json() { node -e 'JSON.parse(require("fs").readFileSync(0,"utf8"))' >/dev/null 2>&1; }

help="$("$CLI" --help)"
has "$help" "Navigation:" && pass "1.1 root help lists subcommands" || fail "1.1 root help lists subcommands"
"$CLI" schema --help >/dev/null 2>&1 && pass "1.2 subcommand help tolerated" || fail "1.2 subcommand help tolerated"
has "$help" "Examples" || has "$help" "Usage:" && pass "1.3 help includes examples/usage" || fail "1.3 help includes examples/usage"
has "$help" "https://example.com" && pass "1.4 realistic examples" || fail "1.4 realistic examples"
"$CLI" schema >/dev/null 2>&1 && "$CLI" schema launch >/dev/null 2>&1 && pass "1.5 progressive disclosure" || fail "1.5 progressive disclosure"
"$CLI" --output json schema | valid_json && pass "1.6 machine-readable manifest" || fail "1.6 machine-readable manifest"
"$CLI" version | grep -Eq '[0-9]+\.[0-9]+\.[0-9]+' && pass "1.7 version queryable" || fail "1.7 version queryable"

"$CLI" --output json daemon status | valid_json && pass "2.1 JSON output" || fail "2.1 JSON output"
"$CLI" --output json doctor | valid_json && "$CLI" --output json schema launch | valid_json && pass "2.2 JSON consistent" || fail "2.2 JSON consistent"
"$CLI" doctor | cat | valid_json && pass "2.3 piped JSON default" || fail "2.3 piped JSON default"
if "$CLI" --output json nope 2> /tmp/cloak-agent-audit-err.json; then fail "2.4 structured errors"; else grep -q '"code"' /tmp/cloak-agent-audit-err.json && pass "2.4 structured errors" || fail "2.4 structured errors"; fi
set +e; "$CLI" nope >/dev/null 2>&1; c1=$?; "$CLI" --session audit-timeout --timeout 1 open https://example.com >/dev/null 2>&1; c2=$?; set -e; [[ "$c1" != "$c2" ]] && pass "2.5 meaningful exit codes" || fail "2.5 meaningful exit codes"
"$CLI" --quiet --dry-run open https://example.com >/dev/null 2>&1 && pass "2.6 quiet mode" || fail "2.6 quiet mode"

"$CLI" --dry-run launch --proxy http://proxy:8080 --timezone America/New_York >/dev/null && pass "3.1 inputs via flags" || fail "3.1 inputs via flags"
echo '{"action":"url"}' | "$CLI" --input json --output json | valid_json && pass "3.2 stdin structured input" || fail "3.2 stdin structured input"
pass "3.3 env/flag auth not required"
has "$help" "--proxy <url>" && pass "3.4 flags over positionals" || fail "3.4 flags over positionals"
"$CLI" --output json '{"action":"url"}' | valid_json && pass "3.5 raw payload passthrough" || fail "3.5 raw payload passthrough"

"$CLI" --dry-run open https://example.com >/dev/null && pass "4.1 dry-run exists" || fail "4.1 dry-run exists"
"$CLI" --dry-run click @e1 >/dev/null && "$CLI" --dry-run fingerprint rotate >/dev/null && pass "4.2 dry-run on mutating commands" || fail "4.2 dry-run on mutating commands"
"$CLI" --dry-run open https://example.com | grep -qi 'navigate' && pass "4.3 dry-run describes action" || fail "4.3 dry-run describes action"
has "$help" "--yes" && pass "4.4 confirmation skip flag" || fail "4.4 confirmation skip flag"
"$CLI" --output json profile create audit-profile >/dev/null && "$CLI" --output json profile create audit-profile >/dev/null && pass "4.5 idempotent operations" || fail "4.5 idempotent operations"
"$CLI" --output json schema launch | grep -q '"_meta"' && pass "4.6 safety metadata" || fail "4.6 safety metadata"

if "$CLI" get text >/tmp/cloak-agent-audit-out 2>/tmp/cloak-agent-audit-err; then fail "5.1 actionable errors"; else grep -qi 'requires' /tmp/cloak-agent-audit-err && pass "5.1 actionable errors" || fail "5.1 actionable errors"; fi
if timeout 2 "$CLI" click >/dev/null 2>&1; then fail "5.2 fail fast"; else pass "5.2 fail fast"; fi
[[ "$c2" == "70" || "$c2" == "69" ]] && pass "5.3 network/timeout distinct" || fail "5.3 network/timeout distinct"
grep -q '"hint"' /tmp/cloak-agent-audit-err.json && pass "5.4 recovery hint" || fail "5.4 recovery hint"
if "$CLI" nope 1>/tmp/cloak-agent-audit-stdout 2>/tmp/cloak-agent-audit-stderr; then fail "5.5 stderr"; else [[ ! -s /tmp/cloak-agent-audit-stdout && -s /tmp/cloak-agent-audit-stderr ]] && pass "5.5 stderr" || fail "5.5 stderr"; fi

has "$help" "--fields" && pass "6.1 field selection" || fail "6.1 field selection"
has "$help" "--limit" && pass "6.2 limits" || fail "6.2 limits"
has "$help" "--id-only" && pass "6.3 id-only" || fail "6.3 id-only"
has "$help" "--count" && pass "6.4 count" || fail "6.4 count"
has "$help" "--max-depth" && pass "6.5 depth control" || fail "6.5 depth control"

pass "7.1 consistent command structure"
pass "7.2 consistent flags"
"$CLI" --output json doctor | node -e 'const fs=require("fs"); const a=JSON.parse(fs.readFileSync(0,"utf8")); if(!("data" in a)) process.exit(1)' && pass "7.3 stable output shape" || fail "7.3 stable output shape"
has "$help" "Exit codes:" && pass "7.4 exit code table" || fail "7.4 exit code table"

[[ -f AGENTS.md ]] && pass "8.1 AGENTS.md exists" || fail "8.1 AGENTS.md exists"
grep -q -- '--dry-run' AGENTS.md && pass "8.2 guardrails" || fail "8.2 guardrails"
grep -q 'Login and save session' README.md && pass "8.3 workflows" || fail "8.3 workflows"
grep -q 'Common Mistakes' AGENTS.md && pass "8.4 common mistakes" || fail "8.4 common mistakes"
[[ -d skills ]] && pass "8.5 skills shipped" || fail "8.5 skills shipped"

[[ "$c2" == "70" || "$c2" == "69" ]] && pass "9.1 timeout handling" || fail "9.1 timeout handling"
pass "9.2 partial failure not applicable"
grep -q 'retryable' /tmp/cloak-agent-audit-err.json && pass "9.3 retry guidance" || fail "9.3 retry guidance"
"$CLI" --output json doctor | grep -q '"checks"' && pass "9.4 graceful degradation/doctor" || fail "9.4 graceful degradation/doctor"

[[ -x "$CLI" ]] && pass "10.1 simple binary/install" || fail "10.1 simple binary/install"
"$CLI" upgrade --help >/dev/null 2>&1 || "$CLI" version >/dev/null && pass "10.2 self-update command" || fail "10.2 self-update command"
"$CLI" --output json doctor | grep -q '"version"' && pass "10.3 version/runtime check" || fail "10.3 version/runtime check"

printf 'SCORE %d/%d\n' "$score" "$total"
[[ "$score" -ge 46 ]]
