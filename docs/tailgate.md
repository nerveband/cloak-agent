# Per-browser tailgate routing

Tailgate routes one named cloak-agent session through an SSH dynamic SOCKS tunnel. It does not change the host route table or the Tailscale exit-node setting. Direct and routed work should use different `--session` names.

Create `~/.cloak-agent/tailgate.json` with mode `0600`:

```json
{
  "routes": {
    "default": {
      "sshHost": "an-alias-from-your-ssh-config",
      "identityFile": "~/.ssh/id_tailgate",
      "knownHostsFile": "~/.ssh/known_hosts"
    }
  }
}
```

Targets and key paths stay in this private local file; do not commit it. `sshHost` is a host name, address, or `user@host`; tailgate deliberately ignores the user's general SSH config so unrelated forwards and agent settings cannot leak into the tunnel. Tailgate requires OpenSSH, key-only noninteractive authentication, and an existing strict host-key entry. SOCKS and SSH control endpoints bind locally.

```bash
cloak-agent --session routed --tailgate launch --profile account https://example.com
cloak-agent --session routed --tailgate open https://example.com
cloak-agent --session routed tailgate status
cloak-agent --session routed tailgate doctor
cloak-agent --session routed tailgate stop
```

Use `--tailgate-route <name>` for a named route. `CLOAK_AGENT_TAILGATE=1` selects `default`; another non-empty value selects that route name. `CLOAK_AGENT_TAILGATE_CONFIG` overrides the config path.

Routed persistent profiles get a stable route-derived prefix, so `--profile account` cannot share its browser data with the direct `account` profile. A tunnel persists across daemon restarts and `open` restores the routed launch before navigation. Stop it explicitly when finished.

Tailgate supports Linux and macOS. Windows reports an unsupported-platform error. Existing `--proxy` remains available for caller-managed proxies and is deliberately separate from tunnel lifecycle management.
