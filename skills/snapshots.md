---
name: snapshots
description: Rules for efficient page inspection with cloak-agent
invariants:
  - Always use snapshot -i (interactive) unless full page structure is needed
  - Re-snapshot after any navigation or significant DOM change
  - Refs (@e1, @e2) are only valid until next navigation
  - Check token count in response stats before deciding to snapshot again
  - Use snapshot -c (compact) for large pages to reduce tokens
---

# Snapshot Commands

## Quick Reference
- `cloak-agent snapshot -i` -- interactive elements only (recommended default)
- `cloak-agent snapshot -c` -- compact mode (fewer tokens)
- `cloak-agent snapshot -d 3` -- limit tree depth
- `cloak-agent snapshot -s "#main"` -- scope to CSS selector

## Rules
1. **Always snapshot before interacting** -- refs change on navigation
2. **Use -i by default** -- full snapshots waste context window tokens
3. **Check token count** in response to gauge page complexity
4. **Re-snapshot after navigation** -- goto, click on links, form submissions
5. **Scope large pages** -- use -s to limit to a section

## Ref Format
Elements get refs like `@e1`, `@e2`. Use these in subsequent commands:
- `cloak-agent click @e1`
- `cloak-agent fill @e2 "text"`
- `cloak-agent get text @e3`

## Output Example
```
- button "Submit" [ref=e1]
- textbox "Email" [ref=e2]
- link "Sign up" [ref=e3]

Stats: 3 refs, 89 chars, ~23 tokens
```
