// Copyright 2026 Jeremy and contributors. Licensed under Apache-2.0. See LICENSE.
//
// pp:data-source live
// Hand-authored novel command: rank the whole fleet by uptime % for a window,
// aggregating per-monitor stats/uptime no single API call returns together.

package cli

import (
	"context"
	"fmt"
	"sort"

	"peekaping-pp-cli/internal/cliutil"

	"github.com/spf13/cobra"
)

type uptimeEntry struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Type      string  `json:"type"`
	Status    string  `json:"status"`
	UptimePct float64 `json:"uptime_pct"`
	Window    string  `json:"window"`
}

type uptimeView struct {
	Window          string        `json:"window"`
	Below           *float64      `json:"below,omitempty"`
	ScannedMonitors int           `json:"scanned_monitors"`
	Items           []uptimeEntry `json:"items"`
	NoData          []string      `json:"no_data"`
	FetchFailures   []fetchNote   `json:"fetch_failures"`
	Note            string        `json:"note,omitempty"`
}

func validUptimeWindow(w string) bool {
	switch w {
	case "24h", "7d", "30d", "365d":
		return true
	}
	return false
}

func newNovelWorstUptimeCmd(flags *rootFlags) *cobra.Command {
	var flagWindow string
	var flagBelow float64
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "worst-uptime",
		Short: "Rank the fleet by uptime % for a window, or filter below an SLO target",
		Long: cliutil.CleanText(`Rank every monitor by uptime percentage for a window (worst first), or filter
to monitors below an SLO target with --below.

Use this to rank monitors by reliability or filter below an SLO target with
--below. For tag-level rollups use 'tag-health'.`),
		Example:     "  peekaping-pp-cli worst-uptime --window 30d --below 99.9 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("worst-uptime takes no positional arguments"))
			}
			if dryRunOK(flags) {
				return nil
			}
			if !validUptimeWindow(flagWindow) {
				return usageErr(fmt.Errorf("invalid --window %q: use one of 24h, 7d, 30d, 365d", flagWindow))
			}
			belowSet := cmd.Flags().Changed("below")

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			monitors, err := fetchAllMonitors(ctx, c)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			type uptimeResult struct {
				entry  uptimeEntry
				hasVal bool
			}
			results, ferrs := cliutil.FanoutRun(ctx, monitors,
				func(m pkMonitor) string { return m.Name },
				func(ctx2 context.Context, m pkMonitor) (uptimeResult, error) {
					u, uerr := fetchUptime(ctx2, c, m.ID)
					if uerr != nil {
						return uptimeResult{}, uerr
					}
					raw, ok := u.window(flagWindow)
					e := uptimeEntry{ID: m.ID, Name: m.Name, Type: m.Type, Status: pkStatusName(m.Status), Window: flagWindow}
					if ok {
						e.UptimePct = uptimePct(raw)
					}
					return uptimeResult{entry: e, hasVal: ok}, nil
				},
			)

			var items []uptimeEntry
			var noData []string
			for _, r := range results {
				if !r.Value.hasVal {
					noData = append(noData, r.Value.entry.Name)
					continue
				}
				items = append(items, r.Value.entry)
			}
			if belowSet {
				filtered := items[:0]
				for _, e := range items {
					if e.UptimePct < flagBelow {
						filtered = append(filtered, e)
					}
				}
				items = filtered
			}
			sort.Slice(items, func(i, j int) bool {
				if items[i].UptimePct != items[j].UptimePct {
					return items[i].UptimePct < items[j].UptimePct
				}
				return items[i].Name < items[j].Name
			})
			if flagLimit > 0 && len(items) > flagLimit {
				items = items[:flagLimit]
			}
			if items == nil {
				items = []uptimeEntry{}
			}
			if noData == nil {
				noData = []string{}
			}

			fails := make([]fetchNote, 0, len(ferrs))
			for _, e := range ferrs {
				fails = append(fails, fetchNote{Monitor: e.Source, Error: cliutil.CleanText(e.Err.Error())})
			}
			if len(ferrs) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d monitors failed uptime fetch; ranking computed over the rest\n", len(ferrs), len(monitors))
			}

			view := uptimeView{Window: flagWindow, ScannedMonitors: len(monitors), Items: items, NoData: noData, FetchFailures: fails}
			if belowSet {
				b := flagBelow
				view.Below = &b
				if len(items) == 0 {
					view.Note = fmt.Sprintf("no monitor is below %.3f%% uptime over %s", flagBelow, flagWindow)
				}
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(items) == 0 {
				if view.Note != "" {
					fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), "No uptime data available.")
				}
				return nil
			}
			rows := make([]map[string]any, 0, len(items))
			for _, e := range items {
				rows = append(rows, map[string]any{
					"name": e.Name, "type": e.Type, "status": e.Status,
					"uptime_pct": fmt.Sprintf("%.3f", e.UptimePct),
				})
			}
			return printAutoTable(cmd.OutOrStdout(), rows)
		},
	}
	cmd.Flags().StringVar(&flagWindow, "window", "30d", "Uptime window: 24h, 7d, 30d, or 365d")
	cmd.Flags().Float64Var(&flagBelow, "below", 0, "Only show monitors below this uptime percentage (SLO target)")
	cmd.Flags().IntVar(&flagLimit, "limit", 20, "Maximum monitors to return")
	return cmd
}
