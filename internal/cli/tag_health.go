// Copyright 2026 Jeremy and contributors. Licensed under Apache-2.0. See LICENSE.
//
// pp:data-source live
// Hand-authored novel command: roll up monitor status and average uptime
// grouped by tag (e.g. prod vs staging) — a join no single API call returns.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"peekaping-pp-cli/internal/cliutil"

	"github.com/spf13/cobra"
)

type pkTag struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// tagMonsResult pairs a tag with its monitors so fan-out results stay correctly
// attributed even when some tags' fetches fail and are dropped from results.
type tagMonsResult struct {
	Tag  pkTag
	Mons []pkMonitor
}

type tagHealthEntry struct {
	Tag          string   `json:"tag"`
	TagID        string   `json:"tag_id"`
	Monitors     int      `json:"monitors"`
	Up           int      `json:"up"`
	Down         int      `json:"down"`
	Pending      int      `json:"pending"`
	Maintenance  int      `json:"maintenance"`
	CurrentUpPct float64  `json:"current_up_pct"`
	AvgUptimePct *float64 `json:"avg_uptime_pct,omitempty"`
}

type tagHealthView struct {
	Window        string           `json:"window"`
	Items         []tagHealthEntry `json:"items"`
	FetchFailures []fetchNote      `json:"fetch_failures"`
	Note          string           `json:"note,omitempty"`
}

func newNovelTagHealthCmd(flags *rootFlags) *cobra.Command {
	var flagWindow string

	cmd := &cobra.Command{
		Use:   "tag-health",
		Short: "Roll up monitor status and average uptime grouped by tag",
		Long: cliutil.CleanText(`Group monitors by tag (e.g. prod vs staging) and report how many are up/down
plus the average uptime over a window — reliability by environment or team at a
glance. To rank individual monitors use 'worst-uptime'.`),
		Example:     "  peekaping-pp-cli tag-health --window 30d --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("tag-health takes no positional arguments"))
			}
			if dryRunOK(flags) {
				return nil
			}
			if !validUptimeWindow(flagWindow) {
				return usageErr(fmt.Errorf("invalid --window %q: use one of 24h, 7d, 30d, 365d", flagWindow))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()

			// 1. Fetch tags.
			rawTags, err := c.Get(ctx, "/tags", nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var tags []pkTag
			if err := json.Unmarshal(unwrapData(rawTags), &tags); err != nil {
				return fmt.Errorf("parsing tags: %w", err)
			}
			if len(tags) == 0 {
				view := tagHealthView{Window: flagWindow, Items: []tagHealthEntry{}, FetchFailures: []fetchNote{}, Note: "no tags defined"}
				if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
					return printJSONFiltered(cmd.OutOrStdout(), view, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "No tags defined.")
				return nil
			}

			// 2. For each tag, list its monitors (server-side tag_ids filter).
			// The result carries its own tag so a partial fetch failure (which
			// FanoutRun drops from results) cannot misalign monitors to tags.
			tagMons, ferrs := cliutil.FanoutRun(ctx, tags,
				func(t pkTag) string { return t.Name },
				func(ctx2 context.Context, t pkTag) (tagMonsResult, error) {
					_, limit := analyticsLimits()
					raw, gerr := c.Get(ctx2, "/monitors", map[string]string{
						"tag_ids": t.ID,
						"limit":   fmt.Sprintf("%d", limit),
						"page":    "0",
					})
					if gerr != nil {
						return tagMonsResult{}, gerr
					}
					var mons []pkMonitor
					if jerr := json.Unmarshal(unwrapData(raw), &mons); jerr != nil {
						return tagMonsResult{}, fmt.Errorf("parsing monitors for tag %q: %w", t.Name, jerr)
					}
					return tagMonsResult{Tag: t, Mons: mons}, nil
				},
			)

			// 3. Collect unique monitor ids for an uptime pass (skipped under dogfood).
			uniqueIDs := map[string]bool{}
			tagToMons := map[string][]pkMonitor{}
			for _, r := range tagMons {
				tagToMons[r.Value.Tag.ID] = r.Value.Mons
				for _, m := range r.Value.Mons {
					uniqueIDs[m.ID] = true
				}
			}
			uptimeByID := map[string]float64{}
			if !cliutil.IsDogfoodEnv() {
				ids := make([]string, 0, len(uniqueIDs))
				for id := range uniqueIDs {
					ids = append(ids, id)
				}
				upRes, _ := cliutil.FanoutRun(ctx, ids,
					func(id string) string { return id },
					func(ctx2 context.Context, id string) (float64, error) {
						u, uerr := fetchUptime(ctx2, c, id)
						if uerr != nil {
							return 0, uerr
						}
						v, ok := u.window(flagWindow)
						if !ok {
							return 0, fmt.Errorf("no %s uptime", flagWindow)
						}
						return uptimePct(v), nil
					},
				)
				for _, r := range upRes {
					uptimeByID[r.Source] = r.Value
				}
			}

			// 4. Aggregate per tag.
			var items []tagHealthEntry
			for i := range tags {
				t := tags[i]
				mons := tagToMons[t.ID]
				e := tagHealthEntry{Tag: t.Name, TagID: t.ID, Monitors: len(mons)}
				var upSum float64
				var upN int
				for _, m := range mons {
					switch m.Status {
					case pkStatusUp:
						e.Up++
					case pkStatusDown:
						e.Down++
					case pkStatusPending:
						e.Pending++
					case pkStatusMaintenance:
						e.Maintenance++
					}
					if v, ok := uptimeByID[m.ID]; ok {
						upSum += v
						upN++
					}
				}
				if len(mons) > 0 {
					e.CurrentUpPct = float64(e.Up) / float64(len(mons)) * 100
				}
				if upN > 0 {
					avg := upSum / float64(upN)
					e.AvgUptimePct = &avg
				}
				items = append(items, e)
			}
			sort.Slice(items, func(i, j int) bool {
				if items[i].Down != items[j].Down {
					return items[i].Down > items[j].Down
				}
				return items[i].Tag < items[j].Tag
			})

			fails := make([]fetchNote, 0, len(ferrs))
			for _, e := range ferrs {
				fails = append(fails, fetchNote{Monitor: e.Source, Error: cliutil.CleanText(e.Err.Error())})
			}
			if len(ferrs) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d tags failed monitor fetch\n", len(ferrs), len(tags))
			}

			view := tagHealthView{Window: flagWindow, Items: items, FetchFailures: fails}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			rows := make([]map[string]any, 0, len(items))
			for _, e := range items {
				row := map[string]any{
					"tag": e.Tag, "monitors": e.Monitors, "up": e.Up, "down": e.Down,
					"current_up_pct": fmt.Sprintf("%.1f", e.CurrentUpPct),
				}
				if e.AvgUptimePct != nil {
					row["avg_uptime_pct"] = fmt.Sprintf("%.3f", *e.AvgUptimePct)
				}
				rows = append(rows, row)
			}
			return printAutoTable(cmd.OutOrStdout(), rows)
		},
	}
	cmd.Flags().StringVar(&flagWindow, "window", "30d", "Uptime window: 24h, 7d, 30d, or 365d")
	return cmd
}
