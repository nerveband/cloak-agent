# Interaction playbook

This guide is inspired by and credits [browser-harness](https://github.com/browser-use/browser-harness), especially its agent-facing interaction-skills model: take visual proof first, keep helpers thin, prefer direct browser primitives when normal locators fail, and make recovery steps explicit.

cloak-agent keeps its own architecture: a Go CLI, a long-running Node daemon, Playwright actions, CloakBrowser stealth runtime, structured JSON schemas, and accessibility refs. The practices below borrow what fits that model without copying browser-harness internals.

## Default loop

```bash
cloak-agent open https://example.com
cloak-agent snapshot -i -c --max-depth 3
cloak-agent click @e1
cloak-agent snapshot -i -c --max-depth 3
```

- Re-snapshot after navigation, reloads, modal opens, route changes, and form submissions.
- Use `fill` for inputs unless the site needs keystroke-by-keystroke behavior.
- Keep snapshots compact until you need deeper structure.
- Use `doctor` when daemon, stream, or runtime state is unclear.

## Screenshots and coordinates

Start with semantic refs when possible, then use screenshots and coordinate input when DOM abstractions are the problem.

```bash
cloak-agent screenshot page.png
cloak-agent mouse move 420 315
cloak-agent mouse down left
cloak-agent mouse up left
```

Coordinate input is useful for canvas UIs, cross-origin iframe surfaces, deeply nested shadow DOM, drag handles, and controls whose accessible role is missing or misleading.

## Iframes and shadow DOM

Refs are generated from the page accessibility tree and may not expose every nested or cross-origin control. When a visible target is missing:

```bash
cloak-agent screenshot page.png
cloak-agent get html "iframe"
cloak-agent cdp Runtime.evaluate '{"expression":"document.elementFromPoint(420,315)?.outerHTML","returnByValue":true}'
```

Use `cdp` as a focused escape hatch for inspection or low-level browser primitives. Keep the method and params narrow, and prefer built-in cloak-agent actions again once you have a stable selector or ref.

## Readiness and network

Pages that render after async work need an explicit wait before the next snapshot.

```bash
cloak-agent wait --load networkidle
cloak-agent wait --fn "document.readyState === 'complete'"
cloak-agent network requests --filter api
cloak-agent snapshot -i
```

Use `wait --load networkidle` for navigation-heavy apps and `wait --fn` for app-specific readiness flags.

## Forms and uploads

```bash
cloak-agent fill @email "user@example.com"
cloak-agent select @country "US"
cloak-agent upload @resume /absolute/path/resume.pdf
cloak-agent click @submit
cloak-agent wait --text "Success"
cloak-agent snapshot -i
```

Prefer a post-submit proof step: visible success text, changed URL, network request, or saved state.

## Tabs and sessions

```bash
cloak-agent tab
cloak-agent tab new https://example.com
cloak-agent tab 1
cloak-agent tab close
cloak-agent doctor
```

`doctor` now asks the daemon for runtime proof when it is running, including active page, tab list, and context count. This follows the browser-harness lesson that "daemon alive" is weaker than "daemon controls a real tab."

