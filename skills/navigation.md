---
name: navigation
description: Rules for navigating pages with cloak-agent
invariants:
  - Always wait for page load after navigation before snapshotting
  - Use wait --load networkidle for SPAs
  - Check URL after navigation to confirm it worked
  - Handle redirects by checking get url after open
---

# Navigation Commands

## Quick Reference
- `cloak-agent open <url>` -- navigate to URL
- `cloak-agent back` -- go back
- `cloak-agent forward` -- go forward
- `cloak-agent reload` -- reload page
- `cloak-agent close` -- close browser

## Workflow Pattern
1. `cloak-agent open https://example.com`
2. `cloak-agent snapshot -i` -- see interactive elements
3. Interact with refs
4. Re-snapshot after any navigation

## Waiting
- `cloak-agent wait --load networkidle` -- wait for network idle (good for SPAs)
- `cloak-agent wait --url "/dashboard"` -- wait for URL change
- `cloak-agent wait @e1` -- wait for specific element
- `cloak-agent wait 2000` -- wait N milliseconds

## Auth Pattern
```bash
cloak-agent open https://app.com/login
cloak-agent snapshot -i
cloak-agent fill @e1 "user@email.com"
cloak-agent fill @e2 "password"
cloak-agent click @e3
cloak-agent wait --url "/dashboard"
cloak-agent state save auth.json
```
