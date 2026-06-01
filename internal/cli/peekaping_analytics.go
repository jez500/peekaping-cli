// Copyright 2026 Jeremy and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored shared helpers for the Peekaping reliability-analytics commands
// (down, flapping, worst-uptime, certs, incidents, tag-health). These commands
// fan out across per-monitor endpoints (heartbeats, stats/uptime, tls) and
// aggregate/rank in Go — the cross-endpoint leverage no single API call gives.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"peekaping-pp-cli/internal/client"
	"peekaping-pp-cli/internal/cliutil"
)

// Peekaping shared.MonitorStatus enum (integer-coded).
const (
	pkStatusDown        = 0
	pkStatusUp          = 1
	pkStatusPending     = 2
	pkStatusMaintenance = 3
)

func pkStatusName(s int) string {
	switch s {
	case pkStatusDown:
		return "down"
	case pkStatusUp:
		return "up"
	case pkStatusPending:
		return "pending"
	case pkStatusMaintenance:
		return "maintenance"
	default:
		return fmt.Sprintf("unknown(%d)", s)
	}
}

// pkMonitor is the subset of monitor.Model the analytics commands consume.
type pkMonitor struct {
	ID            string       `json:"id"`
	Name          string       `json:"name"`
	Type          string       `json:"type"`
	Active        bool         `json:"active"`
	Status        int          `json:"status"`
	Interval      int          `json:"interval"`
	LastHeartbeat *pkHeartbeat `json:"last_heartbeat,omitempty"`
}

// pkHeartbeat is the subset of shared.HeartBeatModel / heartbeat.Model used here.
type pkHeartbeat struct {
	ID        string `json:"id"`
	MonitorID string `json:"monitor_id"`
	Status    int    `json:"status"`
	Msg       string `json:"msg"`
	Ping      int    `json:"ping"`
	Important bool   `json:"important"`
	DownCount int    `json:"down_count"`
	Retries   int    `json:"retries"`
	Duration  int    `json:"duration"`
	Time      string `json:"time"`
}

// fetchNote records one source's failure in a command's JSON envelope so a
// partial fan-out failure is visible to the caller, not silently dropped.
type fetchNote struct {
	Monitor string `json:"monitor"`
	Error   string `json:"error"`
}

// unwrapData strips the Peekaping {data, message} envelope. If the body is not
// an envelope (already unwrapped), it is returned unchanged.
func unwrapData(raw json.RawMessage) json.RawMessage {
	var env struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &env); err == nil && len(env.Data) > 0 {
		return env.Data
	}
	return raw
}

// analyticsLimits returns (maxPages, limit) for paging, curtailed under live
// dogfood so the fleet-wide scan fits the matrix's per-command timeout.
func analyticsLimits() (maxPages, limit int) {
	if cliutil.IsDogfoodEnv() {
		return 1, 25
	}
	return 50, 100
}

// fetchAllMonitors pages through /monitors and returns the full fleet.
// Peekaping list pagination is 0-based: page 0 is the first page.
func fetchAllMonitors(ctx context.Context, c *client.Client) ([]pkMonitor, error) {
	maxPages, limit := analyticsLimits()
	var all []pkMonitor
	for page := 0; page < maxPages; page++ {
		raw, err := c.Get(ctx, "/monitors", map[string]string{
			"page":  fmt.Sprintf("%d", page),
			"limit": fmt.Sprintf("%d", limit),
		})
		if err != nil {
			return nil, err
		}
		var batch []pkMonitor
		if err := json.Unmarshal(unwrapData(raw), &batch); err != nil {
			return nil, fmt.Errorf("parsing monitors page %d: %w", page, err)
		}
		all = append(all, batch...)
		if len(batch) < limit {
			break
		}
	}
	return all, nil
}

// fetchHeartbeats fetches heartbeats for one monitor. importantOnly maps to the
// server-side ?important=true filter; limit caps the page size.
func fetchHeartbeats(ctx context.Context, c *client.Client, monitorID string, importantOnly bool, limit int) ([]pkHeartbeat, error) {
	params := map[string]string{
		"limit": fmt.Sprintf("%d", limit),
		"page":  "0",
	}
	if importantOnly {
		params["important"] = "true"
	}
	raw, err := c.Get(ctx, "/monitors/"+url.PathEscape(monitorID)+"/heartbeats", params)
	if err != nil {
		return nil, err
	}
	var hbs []pkHeartbeat
	if err := json.Unmarshal(unwrapData(raw), &hbs); err != nil {
		return nil, fmt.Errorf("parsing heartbeats: %w", err)
	}
	return hbs, nil
}

// latestHeartbeat returns the heartbeat with the greatest parsed timestamp from
// a small recent page, so callers get the current state regardless of the
// server's default ordering.
func latestHeartbeat(ctx context.Context, c *client.Client, monitorID string) (*pkHeartbeat, error) {
	hbs, err := fetchHeartbeats(ctx, c, monitorID, false, 25)
	if err != nil {
		return nil, err
	}
	var latest *pkHeartbeat
	var latestT time.Time
	for i := range hbs {
		t, ok := parseHeartbeatTime(hbs[i].Time)
		if latest == nil || (ok && t.After(latestT)) {
			h := hbs[i]
			latest = &h
			latestT = t
		}
	}
	return latest, nil
}

// pkUptimeStats mirrors monitor.CustomUptimeStatsDto. Values are uptime ratios
// in [0,1] (Uptime-Kuma lineage); uptimePct normalizes either scale to percent.
type pkUptimeStats struct {
	H24  *float64 `json:"24h"`
	D7   *float64 `json:"7d"`
	D30  *float64 `json:"30d"`
	D365 *float64 `json:"365d"`
}

func (u pkUptimeStats) window(w string) (float64, bool) {
	switch w {
	case "24h":
		return derefF(u.H24)
	case "7d":
		return derefF(u.D7)
	case "30d":
		return derefF(u.D30)
	case "365d":
		return derefF(u.D365)
	default:
		return 0, false
	}
}

func derefF(p *float64) (float64, bool) {
	if p == nil {
		return 0, false
	}
	return *p, true
}

// uptimePct normalizes an uptime value to a percentage. Peekaping returns a
// ratio in [0,1]; values already on a 0-100 scale are passed through.
func uptimePct(v float64) float64 {
	if v <= 1.0 {
		return v * 100
	}
	return v
}

func fetchUptime(ctx context.Context, c *client.Client, monitorID string) (pkUptimeStats, error) {
	raw, err := c.Get(ctx, "/monitors/"+url.PathEscape(monitorID)+"/stats/uptime", nil)
	if err != nil {
		return pkUptimeStats{}, err
	}
	var u pkUptimeStats
	if err := json.Unmarshal(unwrapData(raw), &u); err != nil {
		return pkUptimeStats{}, fmt.Errorf("parsing uptime: %w", err)
	}
	return u, nil
}

// parseHeartbeatTime parses Peekaping heartbeat timestamps tolerantly.
func parseHeartbeatTime(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	formats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.999Z07:00",
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
