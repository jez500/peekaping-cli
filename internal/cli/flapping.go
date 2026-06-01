// Copyright 2026 Jeremy and contributors. Licensed under Apache-2.0. See LICENSE.
//
// pp:data-source live
// Hand-authored novel command: rank monitors by state-change frequency over a
// window using the heartbeat `important` flag (state-transition marker).

package cli

import (
	"context"
	"fmt"
	"sort"
	"time"

	"peekaping-pp-cli/internal/cliutil"

	"github.com/spf13/cobra"
)

type flapEntry struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Status      string `json:"status"`
	Transitions int    `json:"transitions"`
	LastMsg     string `json:"last_msg,omitempty"`
	LastChange  string `json:"last_change,omitempty"`
}

type flapView struct {
	Window          string      `json:"window"`
	MinTransitions  int         `json:"min_transitions"`
	ScannedMonitors int         `json:"scanned_monitors"`
	Items           []flapEntry `json:"items"`
	FetchFailures   []fetchNote `json:"fetch_failures"`
	Note            string      `json:"note,omitempty"`
}

// countTransitionsInWindow counts state-change (important) heartbeats at or
// after `since`. Pure logic, unit-tested.
func countTransitionsInWindow(hbs []pkHeartbeat, since time.Time) (count int, lastMsg, lastChange string) {
	var lastT time.Time
	for _, h := range hbs {
		if !h.Important {
			continue
		}
		t, ok := parseHeartbeatTime(h.Time)
		if ok && t.Before(since) {
			continue
		}
		count++
		if lastMsg == "" || (ok && t.After(lastT)) {
			lastMsg = cliutil.CleanText(h.Msg)
			lastChange = h.Time
			lastT = t
		}
	}
	return count, lastMsg, lastChange
}

func newNovelFlappingCmd(flags *rootFlags) *cobra.Command {
	var flagSince string
	var flagMinTransitions int
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "flapping",
		Short: "Rank monitors by how often they flip state over a window (the noisiest services)",
		Long: cliutil.CleanText(`Rank monitors by the number of state-change events (the heartbeat 'important'
flag) within a window, surfacing the flakiest, most-oscillating services.

Use this for noisy/oscillating services ranked by state-change frequency. Do
NOT use it for services that are simply down — use 'down' instead.`),
		Example:     "  peekaping-pp-cli flapping --since 7d --min-transitions 4 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("flapping takes no positional arguments"))
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
			monitors, err := fetchAllMonitors(ctx, c)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			hbLimit := 200
			if cliutil.IsDogfoodEnv() {
				hbLimit = 25
			}
			results, ferrs := cliutil.FanoutRun(ctx, monitors,
				func(m pkMonitor) string { return m.Name },
				func(ctx2 context.Context, m pkMonitor) (flapEntry, error) {
					hbs, herr := fetchHeartbeats(ctx2, c, m.ID, true, hbLimit)
					if herr != nil {
						return flapEntry{}, herr
					}
					n, msg, change := countTransitionsInWindow(hbs, cutoff)
					return flapEntry{
						ID: m.ID, Name: m.Name, Type: m.Type, Status: pkStatusName(m.Status),
						Transitions: n, LastMsg: msg, LastChange: change,
					}, nil
				},
			)

			items := make([]flapEntry, 0, len(results))
			for _, r := range results {
				if r.Value.Transitions >= flagMinTransitions {
					items = append(items, r.Value)
				}
			}
			sort.Slice(items, func(i, j int) bool {
				if items[i].Transitions != items[j].Transitions {
					return items[i].Transitions > items[j].Transitions
				}
				return items[i].Name < items[j].Name
			})
			if flagLimit > 0 && len(items) > flagLimit {
				items = items[:flagLimit]
			}

			fails := make([]fetchNote, 0, len(ferrs))
			for _, e := range ferrs {
				fails = append(fails, fetchNote{Monitor: e.Source, Error: cliutil.CleanText(e.Err.Error())})
			}
			if len(ferrs) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d monitors failed heartbeat fetch; ranking computed over the rest\n", len(ferrs), len(monitors))
			}

			view := flapView{
				Window: flagSince, MinTransitions: flagMinTransitions,
				ScannedMonitors: len(monitors), Items: items, FetchFailures: fails,
			}
			if len(items) == 0 {
				view.Note = fmt.Sprintf("no monitor changed state at least %d times in the window; lower --min-transitions or widen --since", flagMinTransitions)
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(items) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				return nil
			}
			rows := make([]map[string]any, 0, len(items))
			for _, e := range items {
				rows = append(rows, map[string]any{
					"name": e.Name, "type": e.Type, "status": e.Status,
					"transitions": e.Transitions, "last_msg": truncate(e.LastMsg, 50),
				})
			}
			return printAutoTable(cmd.OutOrStdout(), rows)
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "7d", "Look-back window (e.g. 24h, 7d, 1w)")
	cmd.Flags().IntVar(&flagMinTransitions, "min-transitions", 4, "Minimum state changes in the window to be considered flapping")
	cmd.Flags().IntVar(&flagLimit, "limit", 20, "Maximum monitors to return")
	return cmd
}
