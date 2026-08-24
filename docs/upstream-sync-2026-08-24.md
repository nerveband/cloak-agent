# Upstream and agent-browser parity — 2026-08-24

Primary comparison points:

- agent-browser main commit `8ec2bb0` (0.35.0 release preparation)
- agent-browser 0.34.0 release commit `548b159`
- prior cloak-agent parity baseline: agent-browser 0.33.2

## CloakBrowser lock

- npm wrapper: `cloakbrowser` 0.5.8.
- verified free Linux x64 runtime: Chromium 146.0.7680.177.5.
- `playwright-core` 1.62.1 and `ws` 8.21.2 remain locked.

## Imported

agent-browser 0.35.0 adds private proxy CA trust for locally launched Chromium
on Linux. cloak-agent implements the same security contract in its Go and
CloakBrowser architecture:

- `--ca-cert <path>` and `CLOAK_AGENT_CA_CERT` accept DER certificates and PEM
  bundles.
- Certificates are validated and imported with `certutil` into a mode-0700,
  temporary NSS home used only by the launched browser.
- Hostname, certificate validity, and unrelated-authority verification stay
  enabled. The implementation does not use `--ignore-https-errors`.
- `--profile` and `--ignore-https-errors` conflict with private CA trust.
- `--no-ca-cert` and `CLOAK_AGENT_CLEAR_CA_CERT=1` explicitly select ordinary
  browser trust for the new launch.

## Not applicable

agent-browser 0.34.0 added persistent target binding and `--pin-tab` for named
sessions attached to one shared external Chrome through `--cdp` or
`--auto-connect`. cloak-agent launches a separate CloakBrowser context per
session and exposes CDP only as an action against that owned page. There is no
shared external Chrome target to bind, so importing `--pin-tab` would create a
flag with no valid behavior.

The 0.34 Remote Agent Browser guide and 0.35 protected Vercel deployment skill
belong to agent-browser's provider, MCP, and hosted-deployment layers.
cloak-agent intentionally has no provider/MCP layer and does not copy those
operational guides.

The 0.34 Chrome-version doctor fix is also not applicable. cloak-agent doctor
does not spawn Chrome to obtain a version, so it has no corresponding hanging
subprocess.

## Verification boundary

Linux x64 is the native runtime baseline. Darwin and Windows archives are
cross-built and inspected; they are not native runtime tests. Private CA trust
fails closed outside Linux and reports the required `certutil` package when the
binary is absent.
