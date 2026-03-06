---
name: interaction
description: Rules for interacting with page elements
invariants:
  - Always use fill instead of type for input fields (fill clears first)
  - Use type only when simulating real keystroke-by-keystroke input is needed
  - Always snapshot first to get valid refs
  - If click fails with "blocked by overlay", dismiss modals/cookie banners first
  - Use --dry-run for mutations when unsure
---

# Interaction Commands

## Quick Reference
- `cloak-agent click @e1` -- click element
- `cloak-agent fill @e2 "text"` -- clear and fill input
- `cloak-agent type @e2 "text"` -- type keystroke by keystroke
- `cloak-agent press Enter` -- press key
- `cloak-agent hover @e1` -- hover over element
- `cloak-agent check @e1` -- check checkbox
- `cloak-agent select @e1 "value"` -- select dropdown option
- `cloak-agent scroll down 500` -- scroll page

## Form Submission Pattern
```bash
cloak-agent open https://example.com/form
cloak-agent snapshot -i
# Output: textbox "Email" [ref=e1], textbox "Password" [ref=e2], button "Submit" [ref=e3]
cloak-agent fill @e1 "user@example.com"
cloak-agent fill @e2 "password123"
cloak-agent click @e3
cloak-agent wait --load networkidle
cloak-agent snapshot -i  # check result
```

## Troubleshooting
- **"blocked by overlay"** -- dismiss cookie banners or modals first
- **"matched N elements"** -- re-snapshot to get unique refs
- **"not visible"** -- try `cloak-agent scrollintoview @e1` first
- **"timed out"** -- page may still be loading, try `wait --load networkidle`

## Semantic Locators (alternative to refs)
```bash
cloak-agent find role button click --name "Submit"
cloak-agent find text "Sign In" click
cloak-agent find label "Email" fill "user@test.com"
```
