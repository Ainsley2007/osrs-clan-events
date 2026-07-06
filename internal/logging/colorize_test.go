package logging

import (
	"strings"
	"testing"
)

func TestColorizeLine_scheduler(t *testing.T) {
	cases := []struct {
		line    string
		plain   bool // if true, expect no ANSI
		contain string
	}{
		{"[Guild 1] Starting rollover (2 events: botw, sotw)", false, "Starting rollover"},
		{"[Guild 1] Rollover commit complete (2 events)", false, "Rollover commit"},
		{"[Guild 1] Deferring rollover: transient snapshot failures (429/5xx)", false, "Deferring"},
		{"[Guild 1] CRITICAL: failed to commit new sotw event: boom", false, "CRITICAL"},
		{"Starting scheduler...", false, "Starting scheduler"},
		{"Hourly snapshot update: 12 unique accounts, 2 events across 1 guilds, completed in 1.2s", false, "Hourly snapshot"},
		{"Updating snapshots for 2 active events", false, "Updating snapshots"},
		{"Error getting expired active events: timeout", false, "Error getting"},
		{"Found 1 expired active events to process on startup", false, "expired"},
		{"Completion checker stopped", false, "stopped"},
	}

	for _, tc := range cases {
		got := colorizeLine(tc.line)
		if tc.plain {
			if got != tc.line {
				t.Fatalf("expected plain line for %q, got %q", tc.line, got)
			}
			continue
		}
		if got == tc.line {
			t.Fatalf("expected color for %q", tc.line)
		}
		if !strings.Contains(got, tc.contain) {
			t.Fatalf("colored line should still contain %q: %s", tc.contain, got)
		}
		if !strings.Contains(got, "\033[") {
			t.Fatalf("expected ANSI in %q", got)
		}
	}
}

func TestColorizeLine_guild(t *testing.T) {
	got := colorizeLine("[Guild 1] Renamed category 123 to ╔═══SOTW - Crafting═══╗")
	if got == "" || !strings.Contains(got, "Renamed category") {
		t.Fatalf("unexpected: %q", got)
	}
}

func TestHighlight_multiline(t *testing.T) {
	plain := "Starting scheduler...\n[Guild 1] Starting rollover (2 events: botw, sotw)"
	got := highlight(plain, true)
	if !strings.Contains(got, "\033[") {
		t.Fatalf("expected colors in multiline output:\n%s", got)
	}
}
