// Copyright 2026 Jeremy and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Real table-driven tests for the pure cores of the Peekaping analytics
// commands (down, flapping, worst-uptime, certs, incidents, tag-health).

package cli

import (
	"encoding/json"
	"testing"
	"time"
)

func TestPkStatusName(t *testing.T) {
	cases := map[int]string{
		pkStatusDown:        "down",
		pkStatusUp:          "up",
		pkStatusPending:     "pending",
		pkStatusMaintenance: "maintenance",
		9:                   "unknown(9)",
	}
	for in, want := range cases {
		if got := pkStatusName(in); got != want {
			t.Errorf("pkStatusName(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestUnwrapData(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"envelope array", `{"data":[1,2,3],"message":"ok"}`, `[1,2,3]`},
		{"envelope object", `{"data":{"a":1},"message":"ok"}`, `{"a":1}`},
		{"already unwrapped array", `[1,2,3]`, `[1,2,3]`},
		{"no data key", `{"message":"ok"}`, `{"message":"ok"}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := string(unwrapData(json.RawMessage(tc.in)))
			if got != tc.want {
				t.Errorf("unwrapData(%s) = %s, want %s", tc.in, got, tc.want)
			}
		})
	}
}

func TestUptimePct(t *testing.T) {
	tests := []struct {
		in   float64
		want float64
	}{
		{1.0, 100},
		{0.999, 99.9},
		{0.5, 50},
		{0, 0},
		{99.9, 99.9}, // already a percentage
		{100, 100},
	}
	for _, tc := range tests {
		if got := uptimePct(tc.in); got != tc.want {
			t.Errorf("uptimePct(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestValidUptimeWindow(t *testing.T) {
	for _, w := range []string{"24h", "7d", "30d", "365d"} {
		if !validUptimeWindow(w) {
			t.Errorf("validUptimeWindow(%q) = false, want true", w)
		}
	}
	for _, w := range []string{"", "1h", "12h", "90d", "garbage"} {
		if validUptimeWindow(w) {
			t.Errorf("validUptimeWindow(%q) = true, want false", w)
		}
	}
}

func TestParseHeartbeatTime(t *testing.T) {
	good := []string{
		"2026-06-01T05:00:00Z",
		"2026-06-01T05:00:00.123Z",
		"2026-06-01T05:00:00+10:00",
		"2026-06-01 05:00:00",
	}
	for _, s := range good {
		if _, ok := parseHeartbeatTime(s); !ok {
			t.Errorf("parseHeartbeatTime(%q) failed, want ok", s)
		}
	}
	if _, ok := parseHeartbeatTime(""); ok {
		t.Error("parseHeartbeatTime(\"\") = ok, want false")
	}
	if _, ok := parseHeartbeatTime("not-a-date"); ok {
		t.Error("parseHeartbeatTime(garbage) = ok, want false")
	}
}

func TestCountTransitionsInWindow(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	cutoff := now.Add(-24 * time.Hour)
	hbs := []pkHeartbeat{
		{Important: true, Status: pkStatusDown, Msg: "conn refused", Time: now.Add(-1 * time.Hour).Format(time.RFC3339)},
		{Important: true, Status: pkStatusUp, Msg: "ok", Time: now.Add(-2 * time.Hour).Format(time.RFC3339)},
		{Important: false, Status: pkStatusUp, Msg: "ok", Time: now.Add(-30 * time.Minute).Format(time.RFC3339)}, // not important
		{Important: true, Status: pkStatusDown, Msg: "old", Time: now.Add(-48 * time.Hour).Format(time.RFC3339)}, // before cutoff
	}
	n, lastMsg, lastChange := countTransitionsInWindow(hbs, cutoff)
	if n != 2 {
		t.Errorf("count = %d, want 2 (two important heartbeats in window)", n)
	}
	if lastMsg != "conn refused" {
		t.Errorf("lastMsg = %q, want %q (newest important in window)", lastMsg, "conn refused")
	}
	if lastChange == "" {
		t.Error("lastChange empty, want a timestamp")
	}

	// No important heartbeats → zero.
	calm := []pkHeartbeat{{Important: false, Time: now.Format(time.RFC3339)}}
	if n, _, _ := countTransitionsInWindow(calm, cutoff); n != 0 {
		t.Errorf("count for calm monitor = %d, want 0", n)
	}
}

func TestExtractCertExpiry(t *testing.T) {
	tests := []struct {
		name      string
		in        string
		wantDays  *int
		wantValid string
		wantFound bool
	}{
		{
			name:      "uptime-kuma certInfo shape",
			in:        `{"valid":true,"certInfo":{"validTo":"2026-08-01T00:00:00Z","daysRemaining":61,"issuer":{"CN":"R3"}}}`,
			wantDays:  intPtr(61),
			wantValid: "2026-08-01T00:00:00Z",
			wantFound: true,
		},
		{
			name:      "snake_case days_remaining",
			in:        `{"days_remaining":5,"valid_to":"2026-06-06T00:00:00Z"}`,
			wantDays:  intPtr(5),
			wantValid: "2026-06-06T00:00:00Z",
			wantFound: true,
		},
		{
			name:      "only notAfter date",
			in:        `{"cert":{"notAfter":"2026-07-01T00:00:00Z"}}`,
			wantDays:  nil,
			wantValid: "2026-07-01T00:00:00Z",
			wantFound: true,
		},
		{
			name:      "no cert info",
			in:        `{"valid":false}`,
			wantFound: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			days, validTo, _, found := extractCertExpiry(json.RawMessage(tc.in))
			if found != tc.wantFound {
				t.Fatalf("found = %v, want %v", found, tc.wantFound)
			}
			if !tc.wantFound {
				return
			}
			if (days == nil) != (tc.wantDays == nil) {
				t.Fatalf("days nil mismatch: got %v want %v", days, tc.wantDays)
			}
			if days != nil && *days != *tc.wantDays {
				t.Errorf("days = %d, want %d", *days, *tc.wantDays)
			}
			if validTo != tc.wantValid {
				t.Errorf("validTo = %q, want %q", validTo, tc.wantValid)
			}
		})
	}
}

func intPtr(i int) *int { return &i }
