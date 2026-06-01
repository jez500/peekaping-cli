// Copyright 2026 Jeremy and contributors. Licensed under Apache-2.0. See LICENSE.
//
// pp:data-source live
// Hand-authored novel command: a chronological log of state-change events
// (important heartbeats) across monitors, or one monitor's recent failures.

package cli

import (
	"context"
	"fmt"
	"sort"
	"time"

	"peekaping-pp-cli/internal/cliutil"

	"github.com/spf13/cobra"
)

type incidentEvent struct {
	MonitorID   string `json:"monitor_id"`
	MonitorName string `json:"monitor_name"`
	Status      string `json:"status"`
	Msg         string `json:"msg,omitempty"`
	Ping        int    `json:"ping,omitempty"`
	Time        string `json:"time"`
}

type incidentsView struct {
	Window        string          `json:"window"`
	Events        []incidentEvent `json:"events"`
	FetchFailures []fetchNote     `json:"fetch_failures"`
	Note          string          `json:"note,omitempty"`
}

func newNovelIncidentsCmd(flags *rootFlags) *cobra.Command {
	var flagSince string
	var flagMonitor string
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "incidents",
		Short: "Chronological log of state-change events across monitors",
		Long: cliutil.CleanText(`A time-ordered log of state changes (the heartbeat 'important' flag) across all
monitors, newest first. Scope to one monitor with --monitor <id>.

Use this for a time-ordered log of state changes, or one monitor's latest
failure via --monitor <id> --limit 1. To rank chronic offenders use 'flapping'.`),
		Example:     "  peekaping-pp-cli incidents --since 7d --agent --select monitor_name,status,msg,time",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("incidents takes no positional arguments; use --monitor <id> to scope"))
			}
			if dryRunOK(flags) {
				return nil
			}
			since := 7 * 24 * time.Hour
			if flagSince != "" {
				d, err := cliutil.ParseDurationLoose(flagSince)
				if err != nil {
					return usageErr(fmt.Errorf("invalid --since %q: %w", flagSince, err))
				}
				since = d
			}
			cutoff := time.Now().Add(-since)

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()

			// Resolve the monitor set: one if --monitor, else the whole fleet.
			var monitors []pkMonitor
			if flagMonitor != "" {
				monitors = []pkMonitor{{ID: flagMonitor, Name: flagMonitor}}
				// Best-effort name lookup so output reads nicely.
				if all, lerr := fetchAllMonitors(ctx, c); lerr == nil {
					for _, m := range all {
						if m.ID == flagMonitor {
							monitors[0].Name = m.Name
							break
						}
					}
				}
			} else {
				all, lerr := fetchAllMonitors(ctx, c)
				if lerr != nil {
					return classifyAPIError(lerr, flags)
				}
				monitors = all
			}

			hbLimit := 200
			if cliutil.IsDogfoodEnv() {
				hbLimit = 25
			}
			results, ferrs := cliutil.FanoutRun(ctx, monitors,
				func(m pkMonitor) string { return m.Name },
				func(ctx2 context.Context, m pkMonitor) ([]incidentEvent, error) {
					hbs, herr := fetchHeartbeats(ctx2, c, m.ID, true, hbLimit)
					if herr != nil {
						return nil, herr
					}
					var evs []incidentEvent
					for _, h := range hbs {
						if t, ok := parseHeartbeatTime(h.Time); ok && t.Before(cutoff) {
							continue
						}
						evs = append(evs, incidentEvent{
							MonitorID: m.ID, MonitorName: m.Name, Status: pkStatusName(h.Status),
							Msg: cliutil.CleanText(h.Msg), Ping: h.Ping, Time: h.Time,
						})
					}
					return evs, nil
				},
			)

			var events []incidentEvent
			for _, r := range results {
				events = append(events, r.Value...)
			}
			sort.Slice(events, func(i, j int) bool {
				ti, oki := parseHeartbeatTime(events[i].Time)
				tj, okj := parseHeartbeatTime(events[j].Time)
				if oki && okj && !ti.Equal(tj) {
					return ti.After(tj)
				}
				return events[i].Time > events[j].Time
			})
			if flagLimit > 0 && len(events) > flagLimit {
				events = events[:flagLimit]
			}
			if events == nil {
				events = []incidentEvent{}
			}

			fails := make([]fetchNote, 0, len(ferrs))
			for _, e := range ferrs {
				fails = append(fails, fetchNote{Monitor: e.Source, Error: cliutil.CleanText(e.Err.Error())})
			}
			if len(ferrs) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d monitors failed heartbeat fetch; timeline omits them\n", len(ferrs), len(monitors))
			}

			view := incidentsView{Window: flagSince, Events: events, FetchFailures: fails}
			if len(events) == 0 {
				view.Note = "no state-change events in the window; widen --since"
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(events) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				return nil
			}
			rows := make([]map[string]any, 0, len(events))
			for _, e := range events {
				rows = append(rows, map[string]any{
					"time": e.Time, "monitor": e.MonitorName, "status": e.Status,
					"msg": truncate(e.Msg, 50),
				})
			}
			return printAutoTable(cmd.OutOrStdout(), rows)
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "7d", "Look-back window (e.g. 24h, 7d, 1w)")
	cmd.Flags().StringVar(&flagMonitor, "monitor", "", "Scope to a single monitor ID")
	cmd.Flags().IntVar(&flagLimit, "limit", 50, "Maximum events to return")
	return cmd
}
