---
name: pp-peekaping
description: "The first CLI for Peekaping Trigger phrases: `what's down in peekaping`, `find flapping monitors`, `peekaping uptime report`, `which certs are expiring`, `list my monitors`, `use peekaping`, `run peekaping`."
author: "Jeremy"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - peekaping-pp-cli
    install:
      - kind: go
        bins: [peekaping-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/monitoring/peekaping/cmd/peekaping-pp-cli
---

# Peekaping — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `peekaping-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install peekaping --cli-only
   ```
2. Verify: `peekaping-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/monitoring/peekaping/cmd/peekaping-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Peekaping is API-first but ships no CLI and no MCP server. This wraps every endpoint in agent-native commands and adds a local SQLite store so you can rank the fleet by uptime, find flapping services, surface currently-down monitors with their failure reason, and track expiring TLS certs — all offline and scriptable. Point it at your self-hosted instance with a pk_ API key and a host URL.

## When to Use This CLI

Use this CLI to operate a self-hosted Peekaping instance from the terminal or from an agent: list and manage monitors, find what is down or flapping, rank services by uptime, and watch for expiring TLS certs. It is the right tool when you want scriptable, machine-readable monitoring ops or offline reliability analytics that the web UI and raw API can't produce in one call.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to configure the Peekaping server itself (database backend, deployment) — that is docker-compose/infra, not the API.
- Do not use it for interactive JWT login or 2FA setup — it authenticates with a pk_ API key, not a user session.
- Do not use it to render or host public status pages for end users — it manages status-page config, but the web app serves them.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Reliability analytics over heartbeats
- **`down`** — List every monitor that is currently down, each enriched with its latest failure message and ping.

  _Reach for this first when triaging an alert: it is the one-command answer to 'what is broken right now and why'._

  ```bash
  peekaping-pp-cli down --agent
  ```
- **`flapping`** — Rank monitors by how often they flip state (up/down) over a window, surfacing the noisiest services.

  _Use this to find chronic offenders that pass a point-in-time status check but page repeatedly — the flaky-service question the user explicitly asked for._

  ```bash
  peekaping-pp-cli flapping --since 7d --min-transitions 4 --agent
  ```
- **`worst-uptime`** — Rank the whole fleet by uptime percentage for a window, or filter to monitors below an SLO target.

  _The SLO-review rollup the web UI can't produce: which services breached target last month, ranked._

  ```bash
  peekaping-pp-cli worst-uptime --window 30d --below 99.9 --agent
  ```
- **`incidents`** — A chronological log of state-change events across monitors, or one monitor's recent failures.

  _Answers 'what happened' for a post-incident review or an on-call agent, without clicking through per-monitor heartbeat histories._

  ```bash
  peekaping-pp-cli incidents --since 7d --agent --select monitor_name,status,msg,time
  ```

### Fleet-wide rollups
- **`certs`** — List TLS certificates expiring within a window across all HTTP monitors, sorted by days remaining.

  _Catch certificate rotations before they cause an outage, in one command instead of one TLS call per monitor._

  ```bash
  peekaping-pp-cli certs --within 30d --agent
  ```
- **`tag-health`** — Roll up uptime and down counts grouped by tag, e.g. prod vs staging.

  _See reliability by environment or team at a glance instead of mentally aggregating individual monitors._

  ```bash
  peekaping-pp-cli tag-health --window 30d --agent
  ```

## Command Reference

**api-keys** — Manage api keys

- `peekaping-pp-cli api-keys create` — Create a new API key
- `peekaping-pp-cli api-keys delete` — Delete an API key
- `peekaping-pp-cli api-keys get` — Get a specific API key by ID
- `peekaping-pp-cli api-keys list` — Get all API keys
- `peekaping-pp-cli api-keys list-apikeys` — Get API key configuration including prefix
- `peekaping-pp-cli api-keys update` — Update an API key

**badge** — Manage badge


**health** — Manage health

- `peekaping-pp-cli health` — Returns the current server health

**maintenances** — Manage maintenances

- `peekaping-pp-cli maintenances create` — Create maintenance
- `peekaping-pp-cli maintenances delete` — Delete maintenance
- `peekaping-pp-cli maintenances get` — Get maintenance by ID
- `peekaping-pp-cli maintenances list` — Get maintenances
- `peekaping-pp-cli maintenances update` — Update maintenance
- `peekaping-pp-cli maintenances update-id` — Update maintenance

**monitors** — Manage monitors

- `peekaping-pp-cli monitors create` — Create monitor
- `peekaping-pp-cli monitors delete` — Delete monitor
- `peekaping-pp-cli monitors get` — Get monitor by ID
- `peekaping-pp-cli monitors list` — Get monitors
- `peekaping-pp-cli monitors list-batch` — Get monitors by IDs
- `peekaping-pp-cli monitors update` — Update monitor
- `peekaping-pp-cli monitors update-id` — Update monitor

**notification-channels** — Manage notification channels

- `peekaping-pp-cli notification-channels create` — Create notification channel
- `peekaping-pp-cli notification-channels create-notificationchannels` — Test notification channel
- `peekaping-pp-cli notification-channels delete` — Delete notification channel
- `peekaping-pp-cli notification-channels get` — Get notification channel by ID
- `peekaping-pp-cli notification-channels list` — Get notification channels
- `peekaping-pp-cli notification-channels update` — Update notification channel
- `peekaping-pp-cli notification-channels update-notificationchannels` — Update notification channel

**peekaping-auth** — Manage peekaping auth

- `peekaping-pp-cli peekaping-auth create` — Login admin
- `peekaping-pp-cli peekaping-auth create-2fa` — Disable 2FA (TOTP) for user
- `peekaping-pp-cli peekaping-auth create-2fa-2` — Enable 2FA (TOTP) for user
- `peekaping-pp-cli peekaping-auth create-2fa-3` — Verify 2FA (TOTP) code for user
- `peekaping-pp-cli peekaping-auth create-refresh` — Refresh access token
- `peekaping-pp-cli peekaping-auth create-register` — Register new admin
- `peekaping-pp-cli peekaping-auth update` — Update user password

**peekaping-version** — Manage peekaping version

- `peekaping-pp-cli peekaping-version` — Returns the current server version

**proxies** — Manage proxies

- `peekaping-pp-cli proxies create` — Create proxy
- `peekaping-pp-cli proxies delete` — Delete proxy
- `peekaping-pp-cli proxies get` — Get proxy by ID
- `peekaping-pp-cli proxies list` — Get proxies
- `peekaping-pp-cli proxies update` — Update proxy
- `peekaping-pp-cli proxies update-id` — Update proxy

**settings** — Manage settings

- `peekaping-pp-cli settings delete` — Delete setting by key
- `peekaping-pp-cli settings get` — Get setting by key
- `peekaping-pp-cli settings update` — Set setting by key

**status-pages** — Manage status pages

- `peekaping-pp-cli status-pages create` — Create a new status page
- `peekaping-pp-cli status-pages delete` — Delete a status page
- `peekaping-pp-cli status-pages get` — Get a status page by ID
- `peekaping-pp-cli status-pages get-statuspages` — Get a status page by domain name
- `peekaping-pp-cli status-pages get-statuspages-2` — Get a status page by slug
- `peekaping-pp-cli status-pages get-statuspages-3` — Get monitors for a status page by slug with heartbeats and uptime
- `peekaping-pp-cli status-pages get-statuspages-4` — Get monitors for a status page by slug for homepage
- `peekaping-pp-cli status-pages list` — Get all status pages
- `peekaping-pp-cli status-pages update` — Update a status page

**tags** — Manage tags

- `peekaping-pp-cli tags create` — Create tag
- `peekaping-pp-cli tags delete` — Delete tag
- `peekaping-pp-cli tags get` — Get tag by ID
- `peekaping-pp-cli tags list` — Get tags
- `peekaping-pp-cli tags update` — Update tag
- `peekaping-pp-cli tags update-id` — Update tag


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
peekaping-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Triage: what's broken right now

```bash
peekaping-pp-cli down --agent --select name,status,last_msg
```

Currently-down monitors with their latest failure reason, narrowed to the fields an agent needs.

### Weekly SLO review

```bash
peekaping-pp-cli worst-uptime --window 30d --below 99.9
```

Monitors that breached a 99.9% uptime target over the last 30 days, worst first.

### Find the noisy services

```bash
peekaping-pp-cli flapping --since 7d --min-transitions 4
```

Monitors that changed state at least 4 times in the last week — the chronic flappers.

### Cert rotation watch

```bash
peekaping-pp-cli certs --within 30d --csv
```

TLS certs expiring in the next 30 days as CSV for a spreadsheet or ticket.

### Create an HTTP monitor

```bash
peekaping-pp-cli monitors create --dry-run
```

Preview the create-monitor request body before sending it; drop --dry-run to apply.

## Auth Setup
Run `peekaping-pp-cli auth setup` to print the URL and steps for getting a key (add `--launch` to open the URL). Then set:

```bash
export PEEKAPING_API_KEY="<your-key>"
```

Or persist it in `~/.config/peekaping-pp-cli/config.toml`.

Run `peekaping-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  peekaping-pp-cli api-keys list --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and `--ignore-missing` only when a missing delete target should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
peekaping-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
peekaping-pp-cli feedback --stdin < notes.txt
peekaping-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/peekaping-pp-cli/feedback.jsonl`. They are never POSTed unless `PEEKAPING_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `PEEKAPING_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
peekaping-pp-cli profile save briefing --json
peekaping-pp-cli --profile briefing api-keys list
peekaping-pp-cli profile list --json
peekaping-pp-cli profile show briefing
peekaping-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `peekaping-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/monitoring/peekaping/cmd/peekaping-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add peekaping-pp-mcp -- peekaping-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which peekaping-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   peekaping-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `peekaping-pp-cli <command> --help`.
