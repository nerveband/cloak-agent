# Changelog

## 0.3.1 — 2026-08-03

- Fix parsing of the documented `tailgate import --route NAME` form.

## 0.3.0 — 2026-08-03

- Add native per-browser Tailgate routing through a loopback-only SSH dynamic
  SOCKS tunnel, with direct mode, named routes, setup/import/doctor/status/stop,
  strict key-only SSH, recovery state, and route/profile isolation.
- Add XDG-aware fresh-install setup, ssh-agent support, Browser Harness
  compatibility discovery/import, atomic staged installation, and a preserving
  uninstall path.
- Lock CloakBrowser 0.5.3, Playwright Core 1.62.1, and ws 8.21.2; document
  current CloakBrowser/runtime and agent-browser parity decisions.
- Harden structured lifecycle, schema/help/docs alignment, stale-state recovery,
  local SOCKS/control endpoint validation, atomic loopback-port reservations,
  and direct/routed profile recovery across daemon restarts.
