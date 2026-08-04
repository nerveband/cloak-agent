# Tailgate

Tailgate is cloak-agent's name for per-browser Tailscale egress. It starts a
local, dynamic OpenSSH SOCKS tunnel to a Tailscale-connected machine and gives
that proxy only to the selected CloakBrowser session. It does not change the
DevBox route table or the host-wide Tailscale exit-node setting.

Tailgate is a native cloak-agent feature. A Browser Harness installation is not
required. On a host that already has the established Browser Harness wrapper,
the `default` route can be discovered automatically when no native config is
present; the resolved route is used in memory and no Browser Harness process is
started. The explicit `tailgate import` command persists that route for users
who want a normal cloak-agent config.

## Fresh setup

Prerequisites:

- Linux or macOS with OpenSSH (`ssh`), Node.js 20+, npm, and a verified
  `known_hosts` entry for the remote machine.
- A remote SSH account on a machine reachable over Tailscale. That machine
  must have normal internet egress; no Tailscale route or exit-node change is
  made on the local host.
- Either a mode-0600 private key owned by you, or an SSH agent with the key
  already loaded. cloak-agent never copies private key material.

Create a route from a clean clone or an installed binary:

```bash
cloak-agent tailgate setup egress-host \
  --user browser \
  --key ~/.ssh/id_tailgate \
  --known-hosts ~/.ssh/known_hosts
```

For an agent-backed route, omit `--key` and use `--agent`:

```bash
cloak-agent tailgate setup egress-host --agent \
  --known-hosts ~/.ssh/known_hosts
```

An SSH alias can be resolved once with an explicit config file. The resolved
host, user, port, key, and known-hosts paths are stored; unrelated forwards,
proxy jumps, agent forwarding, and local commands are not used at runtime:

```bash
cloak-agent tailgate setup egress-alias \
  --ssh-config ~/.ssh/config --route work
```

The setup command validates the route before writing it. Config is private and
stored at `$CLOAK_AGENT_TAILGATE_CONFIG`, then
`$XDG_CONFIG_HOME/cloak-agent/tailgate.json`, then
`~/.config/cloak-agent/tailgate.json`. A legacy `~/.cloak-agent/tailgate.json`
is read only when the XDG file does not exist. Files are written mode 0600 and
directories mode 0700.

Check prerequisites and the current session:

```bash
cloak-agent --output json --session routed tailgate doctor
cloak-agent --output json --session routed tailgate status
```

Doctor reports only safe booleans and actionable errors. If a tunnel is active,
it performs a local SOCKS5 no-auth handshake. It does not print host names,
addresses, key paths, or credentials.

## Using a route

Select the route for one browser session with a flag or environment variable:

```bash
cloak-agent --session routed --tailgate launch --profile account https://example.com
cloak-agent --session routed --tailgate open https://example.com
cloak-agent --session routed --tailgate snapshot -i
cloak-agent --session routed tailgate stop
```

`--tailgate` selects `default`; `--tailgate-route NAME` selects a named route.
`CLOAK_AGENT_TAILGATE=1` selects `default`, while another non-empty value is a
route name. `--direct` or `CLOAK_AGENT_DIRECT=1` explicitly selects direct
egress and safely stops the session's Tailgate tunnel first. A direct/routed
transition without an explicit choice is rejected.

Every browser-dependent action checks the route, so a snapshot or click after a
daemon restart cannot silently relaunch the browser directly. `schema`,
`runtime_status`, and `profile list` remain local protocol queries and do not
launch a browser.

## Isolation and recovery

Tailgate state lives under the runtime/state directory selected by the normal
XDG-aware cloak-agent paths. Control sockets, locks, and SOCKS listeners bind
to loopback only. A routed profile is namespaced by a hash of the session and
route (`tailgate-…-<profile>`), so it cannot share data with the direct profile
of the same name or with another route. The route descriptor keeps the
user-facing profile name and survives daemon/browser restarts.

Starts are serialized per session. Loopback ports have private atomic
reservations with stale-reservation recovery; collisions, stale markers, dead
control masters, tunnel death, stop failures, and unsafe state fail closed.
`--dry-run` validates configuration and OpenSSH availability without creating
or removing tunnels, profiles, or config. `tailgate stop` verifies the control
master and SOCKS endpoint are gone before deleting recovery state.

Direct launches also keep a private per-session profile descriptor. After a
daemon/browser restart, the next direct browser action restores that named
profile automatically. A routed descriptor records both the tunnel port and
the browser's active proxy port; if the tunnel dies, cloak-agent allocates a
new reserved port and relaunches the browser before continuing. A failed
relaunch remains marked inconsistent rather than falling back to direct
egress.

## Compatibility with Browser Harness

Import is explicit when you want to persist the existing target:

```bash
cloak-agent tailgate import --route browser-harness
```

The importer reads the existing local wrapper/config, resolves its SSH alias,
and writes a normal cloak-agent route. Automatic discovery and import are only
compatibility conveniences; they never become a runtime dependency. After
import, remove or keep Browser Harness independently. A clean clone can always
use `tailgate setup` instead.

## Security model and troubleshooting

Tailgate invokes OpenSSH with a clean config, strict host-key checking,
`BatchMode`, key-only authentication, `ExitOnForwardFailure`, bounded
connect/keepalive timeouts, no agent forwarding, no local/remote commands, and
no TTY. The SSH control socket and dynamic SOCKS endpoint are local-only.

Common fixes:

- `tailgate setup` rejects a key: chmod the key to 0600 and make sure the path
  is readable by the current user; do not paste the key into config.
- Host-key errors: add the remote machine's verified key to the configured
  `known_hosts` file with normal `ssh-keyscan`/SSH administration procedures.
- `stale route marker`: inspect the local session, then run
  `cloak-agent --session NAME tailgate stop`; cloak-agent will not delete a
  descriptor it cannot validate.
- `dead SOCKS tunnel`: stop it, check `tailgate doctor`, and retry. The remote
  Tailscale host must accept key-only SSH and have internet egress.
- Windows returns an explicit unsupported-platform error. Linux is the
  verified release baseline; macOS paths and OpenSSH behavior are kept
  portable but require a native macOS run for final runtime confirmation.

## Clean-clone smoke test

On a new Linux/macOS user account, clone the repository, run `./cloak-agent
install` (or `make install`), run `tailgate setup` with that user's own key and
known-hosts files, then execute `--dry-run tailgate doctor`, `--tailgate launch
https://example.com`, `snapshot -i`, `tailgate status`, `--direct launch`, and
`tailgate stop`. No Browser Harness files, DevBox paths, private aliases, or
pre-existing cloak-agent state are needed.
