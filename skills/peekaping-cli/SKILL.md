---
name: peekaping-cli
description: "Operate a self-hosted Peekaping uptime-monitoring instance from the terminal: list/manage monitors, find what's down or flapping, rank services by uptime, watch expiring TLS certs. Trigger phrases: what's down, find flapping monitors, peekaping uptime report, which certs are expiring, list my monitors, use peekaping."
allowed-tools: "Read Bash"
---

# Peekaping CLI

Agent-native CLI for the [Peekaping](https://peekaping.com) uptime-monitoring API
(self-hosted Uptime Kuma alternative). Wraps every API endpoint as a command and
adds offline reliability analytics no single API call can produce: currently-down
monitors with failure reasons, flapping detection, fleet uptime ranking, and TLS
cert-expiry rollups.

The command is **`peekaping`** (a symlink to the canonical `peekaping-pp-cli`).

## Setup (required before any command)

The CLI authenticates with a Peekaping API key and targets your instance via env vars:

```bash
export PEEKAPING_API_KEY="pk_..."                       # an API key from your Peekaping instance
export PEEKAPING_BASE_URL="https://<your-host>/api/v1"  # your instance; default is http://localhost:8383/api/v1
```

- Get/create a key in the Peekaping web UI (Settings → API Keys), or via `peekaping api-keys create`.
- The key value MUST be complete, including any trailing `=` (it is base64).
- Verify everything with: `peekaping doctor` (checks config, host reachability, auth).

If `peekaping` is not found on `$PATH`, the binary lives at `~/printing-press/library/peekaping`
and installs with `go install ./cmd/peekaping-pp-cli` (then symlink `peekaping` → `peekaping-pp-cli`)
or `npx -y @mvanhorn/printing-press-library install peekaping --cli-only`.

## When to use

Use this CLI to operate a Peekaping instance programmatically: list/create/update/delete
monitors, triage what's down, find flaky services, run SLO/uptime reviews, and track TLS
cert expiry. It's the right tool for scriptable monitoring ops or offline reliability
analytics the web UI can't produce in one call.

**Do not** use it to configure the Peekaping server itself (deployment, DB backend), for
interactive JWT login / 2FA (it uses an API key, not a user session), or to host public
status pages (it manages status-page config; the web app serves them).

## High-value analytics commands (the differentiators)

All are read-only and emit JSON with `--agent` (or `--json`). They fan out across
per-monitor endpoints and aggregate in one call.

| Command | What it answers |
|---------|-----------------|
| `peekaping down [--include-pending] --agent` | What is broken right now, with each monitor's latest failure message + ping. Start here when triaging. |
| `peekaping flapping --since 7d --min-transitions 4 --agent` | Which monitors flip state most (the noisy/flaky offenders), ranked by state-change count. |
| `peekaping worst-uptime --window 30d [--below 99.9] --agent` | Fleet ranked by uptime % (worst first); `--below` filters to monitors under an SLO target. Windows: 24h, 7d, 30d, 365d. |
| `peekaping certs [--within 30d] [--expired] --agent` | TLS certs expiring soon across HTTP monitors, sorted by days remaining. |
| `peekaping incidents --since 7d [--monitor <id>] --agent` | Chronological state-change log across monitors (or one monitor's recent events). |
| `peekaping tag-health --window 30d --agent` | Uptime/down rolled up by tag (e.g. prod vs staging). |

Each emits a JSON envelope with `items`/`events` plus a `fetch_failures` array (partial
fan-out failures are surfaced, never silently dropped) and a `note` explaining empty results.

## Monitor & resource management (full CRUD)

```bash
peekaping monitors list --agent                 # list monitors (supports --q search, --status, --active, --tag-ids)
peekaping monitors get <id> --agent             # one monitor
peekaping monitors create --dry-run             # preview create body; drop --dry-run to apply
peekaping monitors update <id> ...              # update (PUT/PATCH)
peekaping monitors delete <id> --yes            # delete
peekaping monitors heartbeats <id> --agent      # paginated heartbeats
peekaping monitors stats ...                     # ping/up/down points, uptime (24h/30d/365d)
peekaping monitors tls <id> --agent             # TLS cert info
```

`monitors status` filter is an integer enum: **0=Down, 1=Up, 2=Pending, 3=Maintenance**.

Other resource trees (same `list/get/create/update/delete` shape): `tags`, `maintenances`
(+ `pause`/`resume`), `notification-channels` (+ test), `proxies`, `status-pages`,
`api-keys`, `settings`, `badge`. `peekaping health` and `peekaping peekaping-version` hit
the server health/version endpoints.

To find a command from a capability in your own words: `peekaping which "<what you want>"`
(exit 0 = match, exit 2 = no confident match → use `--help`).

## Agent mode

Add `--agent` to any command — expands to `--json --compact --no-input --no-color --yes`.

- Pipeable JSON on stdout, errors on stderr; non-interactive (never prompts).
- `--select` keeps a subset of fields (dotted paths descend into nested structures):
  `peekaping incidents --since 7d --agent --select monitor_name,status,msg,time`
- `--dry-run` previews a request without sending it.
- `--csv` for spreadsheet output; `--limit` to cap list size.
- Read commands wrap output as `{"meta": {"source": "live"|"local"}, "results": <data>}` —
  parse `.results`.

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required (check PEEKAPING_API_KEY) |
| 5 | API error (upstream) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Recipes

```bash
# Triage: what's broken right now, minimal fields
peekaping down --agent --select name,status,last_msg

# Weekly SLO review: monitors under 99.9% over 30 days, worst first
peekaping worst-uptime --window 30d --below 99.9 --agent

# Find chronic flappers in the last week
peekaping flapping --since 7d --min-transitions 4 --agent

# Cert rotation watch as CSV
peekaping certs --within 30d --csv

# Recent state changes for one monitor
peekaping incidents --monitor <monitor-id> --since 30d --limit 5 --agent
```

## Notes for agents

- Always run `peekaping doctor` first if a command returns exit 4 (auth) or a connection
  error — it reports whether the key and base URL are set correctly.
- The analytics commands read live data on every call; there's no required `sync` step for them.
- Empty results are honest (the instance may genuinely have no down/flapping monitors); check
  the `note` field and `scanned_monitors`/`down` counts rather than assuming an error.
