# Peekaping CLI

**The first CLI for Peekaping: full monitor CRUD plus offline down/flapping/uptime analytics no single API call can answer.**

Peekaping is API-first but ships no CLI and no MCP server. This wraps every endpoint in agent-native commands and adds a local SQLite store so you can rank the fleet by uptime, find flapping services, surface currently-down monitors with their failure reason, and track expiring TLS certs — all offline and scriptable. Point it at your self-hosted instance with a pk_ API key and a host URL.

## Install

The recommended path installs both the `peekaping-pp-cli` binary and the `pp-peekaping` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install peekaping
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install peekaping --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install peekaping --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install peekaping --agent claude-code
npx -y @mvanhorn/printing-press-library install peekaping --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/monitoring/peekaping/cmd/peekaping-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/peekaping-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-peekaping --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-peekaping --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-peekaping skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-peekaping. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/peekaping-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `PEEKAPING_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/monitoring/peekaping/cmd/peekaping-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "peekaping": {
      "command": "peekaping-pp-mcp",
      "env": {
        "PEEKAPING_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Quick Start

```bash
# Check config, host reachability, and API-key auth before anything else.
peekaping-pp-cli doctor --dry-run

# Confirm the API key works and see your monitors.
peekaping-pp-cli monitors list --json

# Pull monitors and heartbeats into the local store to unlock the analytics commands.
peekaping-pp-cli sync --resources monitors

# Show everything currently down with its failure reason.
peekaping-pp-cli down --agent

# Find the noisy, oscillating services.
peekaping-pp-cli flapping --since 7d --agent

```

## Unique Features

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

## Usage

Run `peekaping-pp-cli --help` for the full command reference and flag list.

## Commands

### api-keys

Manage api keys

- **`peekaping-pp-cli api-keys create`** - Create a new API key
- **`peekaping-pp-cli api-keys delete`** - Delete an API key
- **`peekaping-pp-cli api-keys get`** - Get a specific API key by ID
- **`peekaping-pp-cli api-keys list`** - Get all API keys
- **`peekaping-pp-cli api-keys list-apikeys`** - Get API key configuration including prefix
- **`peekaping-pp-cli api-keys update`** - Update an API key

### badge

Manage badge


### health

Manage health

- **`peekaping-pp-cli health`** - Returns the current server health

### maintenances

Manage maintenances

- **`peekaping-pp-cli maintenances create`** - Create maintenance
- **`peekaping-pp-cli maintenances delete`** - Delete maintenance
- **`peekaping-pp-cli maintenances get`** - Get maintenance by ID
- **`peekaping-pp-cli maintenances list`** - Get maintenances
- **`peekaping-pp-cli maintenances update`** - Update maintenance
- **`peekaping-pp-cli maintenances update-id`** - Update maintenance

### monitors

Manage monitors

- **`peekaping-pp-cli monitors create`** - Create monitor
- **`peekaping-pp-cli monitors delete`** - Delete monitor
- **`peekaping-pp-cli monitors get`** - Get monitor by ID
- **`peekaping-pp-cli monitors list`** - Get monitors
- **`peekaping-pp-cli monitors list-batch`** - Get monitors by IDs
- **`peekaping-pp-cli monitors update`** - Update monitor
- **`peekaping-pp-cli monitors update-id`** - Update monitor

### notification-channels

Manage notification channels

- **`peekaping-pp-cli notification-channels create`** - Create notification channel
- **`peekaping-pp-cli notification-channels create-notificationchannels`** - Test notification channel
- **`peekaping-pp-cli notification-channels delete`** - Delete notification channel
- **`peekaping-pp-cli notification-channels get`** - Get notification channel by ID
- **`peekaping-pp-cli notification-channels list`** - Get notification channels
- **`peekaping-pp-cli notification-channels update`** - Update notification channel
- **`peekaping-pp-cli notification-channels update-notificationchannels`** - Update notification channel

### peekaping-auth

Manage peekaping auth

- **`peekaping-pp-cli peekaping-auth create`** - Login admin
- **`peekaping-pp-cli peekaping-auth create-2fa`** - Disable 2FA (TOTP) for user
- **`peekaping-pp-cli peekaping-auth create-2fa-2`** - Enable 2FA (TOTP) for user
- **`peekaping-pp-cli peekaping-auth create-2fa-3`** - Verify 2FA (TOTP) code for user
- **`peekaping-pp-cli peekaping-auth create-refresh`** - Refresh access token
- **`peekaping-pp-cli peekaping-auth create-register`** - Register new admin
- **`peekaping-pp-cli peekaping-auth update`** - Update user password

### peekaping-version

Manage peekaping version

- **`peekaping-pp-cli peekaping-version`** - Returns the current server version

### proxies

Manage proxies

- **`peekaping-pp-cli proxies create`** - Create proxy
- **`peekaping-pp-cli proxies delete`** - Delete proxy
- **`peekaping-pp-cli proxies get`** - Get proxy by ID
- **`peekaping-pp-cli proxies list`** - Get proxies
- **`peekaping-pp-cli proxies update`** - Update proxy
- **`peekaping-pp-cli proxies update-id`** - Update proxy

### settings

Manage settings

- **`peekaping-pp-cli settings delete`** - Delete setting by key
- **`peekaping-pp-cli settings get`** - Get setting by key
- **`peekaping-pp-cli settings update`** - Set setting by key

### status-pages

Manage status pages

- **`peekaping-pp-cli status-pages create`** - Create a new status page
- **`peekaping-pp-cli status-pages delete`** - Delete a status page
- **`peekaping-pp-cli status-pages get`** - Get a status page by ID
- **`peekaping-pp-cli status-pages get-statuspages`** - Get a status page by domain name
- **`peekaping-pp-cli status-pages get-statuspages-2`** - Get a status page by slug
- **`peekaping-pp-cli status-pages get-statuspages-3`** - Get monitors for a status page by slug with heartbeats and uptime
- **`peekaping-pp-cli status-pages get-statuspages-4`** - Get monitors for a status page by slug for homepage
- **`peekaping-pp-cli status-pages list`** - Get all status pages
- **`peekaping-pp-cli status-pages update`** - Update a status page

### tags

Manage tags

- **`peekaping-pp-cli tags create`** - Create tag
- **`peekaping-pp-cli tags delete`** - Delete tag
- **`peekaping-pp-cli tags get`** - Get tag by ID
- **`peekaping-pp-cli tags list`** - Get tags
- **`peekaping-pp-cli tags update`** - Update tag
- **`peekaping-pp-cli tags update-id`** - Update tag


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
peekaping-pp-cli api-keys list

# JSON for scripting and agents
peekaping-pp-cli api-keys list --json

# Filter to specific fields
peekaping-pp-cli api-keys list --json --select id,name,status

# Dry run — show the request without sending
peekaping-pp-cli api-keys list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
peekaping-pp-cli api-keys list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
peekaping-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/peekaping-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `PEEKAPING_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `peekaping-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `peekaping-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $PEEKAPING_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 / authentication failed** — Set PEEKAPING_API_KEY to a pk_-prefixed key (create one with 'peekaping-pp-cli api-keys create') and confirm with 'peekaping-pp-cli doctor'.
- **connection refused / no such host** — Set PEEKAPING_BASE_URL to your instance base URL, e.g. https://up.example.com/api/v1.
- **down/flapping/worst-uptime return nothing** — Run 'peekaping-pp-cli sync --resources monitors' first — these commands read the local store, not the live API.
