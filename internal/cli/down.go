// Copyright 2026 Jeremy and contributors. Licensed under Apache-2.0. See LICENSE.
//
// pp:data-source live
// Hand-authored novel command: list monitors currently down, enriched with the
// latest failure reason from their newest heartbeat.

package cli

import (
	"context"
	"fmt"
	"sort"

	"peekaping-pp-cli/internal/cliutil"

	"github.com/spf13/cobra"
)

type downEntry struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Status    string `json:"status"`
	Active    bool   `json:"active"`
	LastMsg   string `json:"last_msg,omitempty"`
	LastPing  int    `json:"last_ping,omitempty"`
	DownCount int    `json:"down_count,omitempty"`
	LastTime  string `json:"last_time,omitempty"`
}

type downView struct {
	Down  int         `json:"down"`
	Items []downEntry `json:"items"`
}

func newNovelDownCmd(flags *rootFlags) *cobra.Command {
	var includePending bool

	cmd := &cobra.Command{
		Use:   "down",
		Short: "List every monitor that is currently down, with its latest failure reason",
		Long: cliutil.CleanText(`List every monitor whose current status is Down, each enriched with the
message, ping and time of its newest heartbeat — the one-command triage answer.

Use this for "what is broken right now and why". Add --include-pending to also
show monitors mid-retry (Pending). To rank chronic offenders over time use
'flapping'; for a state-change log use 'incidents'.`),
		Example:     "  peekaping-pp-cli down --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("down takes no positional arguments"))
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			monitors, err := fetchAllMonitors(ctx, c)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			var downMons []pkMonitor
			for _, m := range monitors {
				if m.Status == pkStatusDown || (includePending && m.Status == pkStatusPending) {
					downMons = append(downMons, m)
				}
			}

			// Enrich each down monitor with its latest heartbeat (failure reason).
			// Every down monitor stays in the output even if its heartbeat fetch
			// fails — the monitor list status is authoritative.
			results, _ := cliutil.FanoutRun(ctx, downMons,
				func(m pkMonitor) string { return m.Name },
				func(ctx2 context.Context, m pkMonitor) (downEntry, error) {
					e := downEntry{ID: m.ID, Name: m.Name, Type: m.Type, Status: pkStatusName(m.Status), Active: m.Active}
					hb := m.LastHeartbeat
					if hb == nil {
						fetched, ferr := latestHeartbeat(ctx2, c, m.ID)
						if ferr != nil {
							e.LastMsg = "heartbeat unavailable: " + cliutil.CleanText(ferr.Error())
							return e, nil
						}
						hb = fetched
					}
					if hb != nil {
						e.LastMsg = cliutil.CleanText(hb.Msg)
						e.LastPing = hb.Ping
						e.DownCount = hb.DownCount
						e.LastTime = hb.Time
					}
					return e, nil
				},
			)

			items := make([]downEntry, 0, len(results))
			for _, r := range results {
				items = append(items, r.Value)
			}
			sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })

			view := downView{Down: len(items), Items: items}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(items) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "All monitors are up.")
				return nil
			}
			rows := make([]map[string]any, 0, len(items))
			for _, e := range items {
				rows = append(rows, map[string]any{
					"name": e.Name, "type": e.Type, "status": e.Status,
					"ping": e.LastPing, "msg": truncate(e.LastMsg, 60),
				})
			}
			return printAutoTable(cmd.OutOrStdout(), rows)
		},
	}
	cmd.Flags().BoolVar(&includePending, "include-pending", false, "Also include monitors that are Pending (mid-retry)")
	return cmd
}
