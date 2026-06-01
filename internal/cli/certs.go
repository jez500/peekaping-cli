// Copyright 2026 Jeremy and contributors. Licensed under Apache-2.0. See LICENSE.
//
// pp:data-source live
// Hand-authored novel command: list TLS certificates expiring soon across HTTP
// monitors, joining per-monitor /tls info into one ranked view.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strings"
	"time"

	"peekaping-pp-cli/internal/cliutil"

	"github.com/spf13/cobra"
)

type certEntry struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	DaysRemaining *int   `json:"days_remaining,omitempty"`
	ValidTo       string `json:"valid_to,omitempty"`
	Issuer        string `json:"issuer,omitempty"`
	Expired       bool   `json:"expired"`
}

type certsView struct {
	WithinDays    int         `json:"within_days"`
	ExpiredOnly   bool        `json:"expired_only"`
	Items         []certEntry `json:"items"`
	NoCert        []string    `json:"no_cert"`
	FetchFailures []fetchNote `json:"fetch_failures"`
	Note          string      `json:"note,omitempty"`
}

// certDateKeys / certDaysKeys / certIssuerKeys are the (lowercased) JSON keys we
// recognize across the Uptime-Kuma TLS lineage and common snake/camel variants.
var (
	certDaysKeys   = map[string]bool{"daysremaining": true, "days_remaining": true, "certexpirydays": true}
	certDateKeys   = map[string]bool{"validto": true, "valid_to": true, "notafter": true, "not_after": true, "expires": true, "expiry": true, "expiresat": true, "expires_at": true}
	certIssuerKeys = map[string]bool{"issuer": true, "issuercn": true, "issuer_cn": true, "issuername": true}
)

// extractCertExpiry walks an arbitrary TLS-info JSON value and pulls out the
// days-remaining (preferred) or a validTo date, plus an issuer string if
// present. Pure logic, unit-tested. Returns found=false when nothing useful is
// present.
func extractCertExpiry(data json.RawMessage) (days *int, validTo, issuer string, found bool) {
	var node any
	if err := json.Unmarshal(data, &node); err != nil {
		return nil, "", "", false
	}
	walkCert(node, &days, &validTo, &issuer)
	found = days != nil || validTo != ""
	return days, validTo, issuer, found
}

func walkCert(node any, days **int, validTo, issuer *string) {
	switch v := node.(type) {
	case map[string]any:
		for k, val := range v {
			lk := strings.ToLower(k)
			switch {
			case certDaysKeys[lk]:
				if f, ok := numericOf(val); ok && *days == nil {
					d := int(math.Round(f))
					*days = &d
				}
			case certDateKeys[lk]:
				if s, ok := val.(string); ok && *validTo == "" {
					*validTo = s
				}
			case certIssuerKeys[lk]:
				if s, ok := val.(string); ok && *issuer == "" {
					*issuer = s
				} else if m, ok := val.(map[string]any); ok && *issuer == "" {
					if cn, ok := m["CN"].(string); ok {
						*issuer = cn
					} else if cn, ok := m["commonName"].(string); ok {
						*issuer = cn
					}
				}
			}
			// Recurse into nested objects/arrays regardless (certInfo lives one level down).
			switch val.(type) {
			case map[string]any, []any:
				walkCert(val, days, validTo, issuer)
			}
		}
	case []any:
		for _, item := range v {
			walkCert(item, days, validTo, issuer)
		}
	}
}

func numericOf(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	}
	return 0, false
}

func newNovelCertsCmd(flags *rootFlags) *cobra.Command {
	var flagWithin string
	var flagExpired bool

	cmd := &cobra.Command{
		Use:   "certs",
		Short: "List TLS certificates expiring soon across monitors, sorted by days remaining",
		Long: cliutil.CleanText(`Join the TLS certificate info of every HTTP monitor into one view, sorted by
days until expiry. Filter to a window with --within (default 30d) or to
already-expired certs with --expired.`),
		Example:     "  peekaping-pp-cli certs --within 30d --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("certs takes no positional arguments"))
			}
			if dryRunOK(flags) {
				return nil
			}
			withinDays := 30
			if flagWithin != "" {
				d, err := cliutil.ParseDurationLoose(flagWithin)
				if err != nil {
					return usageErr(fmt.Errorf("invalid --within %q: %w", flagWithin, err))
				}
				withinDays = int(math.Round(d.Hours() / 24))
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
			var httpMons []pkMonitor
			for _, m := range monitors {
				if strings.EqualFold(m.Type, "http") {
					httpMons = append(httpMons, m)
				}
			}

			type certResult struct {
				entry  certEntry
				hasVal bool
			}
			results, ferrs := cliutil.FanoutRun(ctx, httpMons,
				func(m pkMonitor) string { return m.Name },
				func(ctx2 context.Context, m pkMonitor) (certResult, error) {
					raw, gerr := c.Get(ctx2, "/monitors/"+url.PathEscape(m.ID)+"/tls", nil)
					if gerr != nil {
						return certResult{}, gerr
					}
					days, validTo, issuer, found := extractCertExpiry(unwrapData(raw))
					if !found {
						return certResult{entry: certEntry{ID: m.ID, Name: m.Name}, hasVal: false}, nil
					}
					// Derive days from validTo when the API didn't supply it.
					if days == nil && validTo != "" {
						if t, ok := parseHeartbeatTime(validTo); ok {
							d := int(math.Floor(time.Until(t).Hours() / 24))
							days = &d
						}
					}
					e := certEntry{ID: m.ID, Name: m.Name, DaysRemaining: days, ValidTo: validTo, Issuer: issuer}
					if days != nil && *days < 0 {
						e.Expired = true
					}
					return certResult{entry: e, hasVal: true}, nil
				},
			)

			var items []certEntry
			var noCert []string
			for _, r := range results {
				if !r.Value.hasVal {
					noCert = append(noCert, r.Value.entry.Name)
					continue
				}
				e := r.Value.entry
				if flagExpired {
					if !e.Expired {
						continue
					}
				} else if e.DaysRemaining != nil && *e.DaysRemaining > withinDays {
					continue
				}
				items = append(items, e)
			}
			sort.Slice(items, func(i, j int) bool {
				di, dj := math.MaxInt32, math.MaxInt32
				if items[i].DaysRemaining != nil {
					di = *items[i].DaysRemaining
				}
				if items[j].DaysRemaining != nil {
					dj = *items[j].DaysRemaining
				}
				if di != dj {
					return di < dj
				}
				return items[i].Name < items[j].Name
			})
			if items == nil {
				items = []certEntry{}
			}
			if noCert == nil {
				noCert = []string{}
			}

			fails := make([]fetchNote, 0, len(ferrs))
			for _, e := range ferrs {
				fails = append(fails, fetchNote{Monitor: e.Source, Error: cliutil.CleanText(e.Err.Error())})
			}
			if len(ferrs) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d HTTP monitors failed TLS fetch\n", len(ferrs), len(httpMons))
			}

			view := certsView{WithinDays: withinDays, ExpiredOnly: flagExpired, Items: items, NoCert: noCert, FetchFailures: fails}
			if len(items) == 0 {
				if flagExpired {
					view.Note = "no expired certificates found"
				} else {
					view.Note = fmt.Sprintf("no certificates expiring within %d days", withinDays)
				}
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
				dr := "?"
				if e.DaysRemaining != nil {
					dr = fmt.Sprintf("%d", *e.DaysRemaining)
				}
				rows = append(rows, map[string]any{
					"name": e.Name, "days_remaining": dr, "valid_to": e.ValidTo, "expired": e.Expired,
				})
			}
			return printAutoTable(cmd.OutOrStdout(), rows)
		},
	}
	cmd.Flags().StringVar(&flagWithin, "within", "30d", "Only show certs expiring within this window (e.g. 30d, 1w)")
	cmd.Flags().BoolVar(&flagExpired, "expired", false, "Only show already-expired certificates")
	return cmd
}
