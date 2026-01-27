package services

import (
	"testing"

	"osrs-events/internal/database"
)

func TestBuildBossDisplayName(t *testing.T) {
	// This table verifies that the boss display name is always derived
	// from the configured BossesToTrack list, falling back to MetricJsonID
	// when that list is empty or invalid.
	tests := []struct {
		name       string
		event      *database.Event
		wantOutput string
	}{
		{
			name: "invalid json falls back to metric name",
			event: &database.Event{
				MetricJsonID:  "Vorkath",
				BossesToTrack: "not-json",
			},
			wantOutput: "Vorkath",
		},
		{
			name: "single boss returns the only entry",
			event: &database.Event{
				MetricJsonID:  "Ignored",
				BossesToTrack: `["Zulrah"]`,
			},
			wantOutput: "Zulrah",
		},
		{
			name: "multiple bosses are joined with plus signs",
			event: &database.Event{
				MetricJsonID:  "Ignored",
				BossesToTrack: `["Dagannoth Prime","Dagannoth Rex","Dagannoth Supreme"]`,
			},
			wantOutput: "Dagannoth Prime + Dagannoth Rex + Dagannoth Supreme",
		},
		{
			name: "empty boss list falls back to metric name",
			event: &database.Event{
				MetricJsonID:  "Kal'gerion demon",
				BossesToTrack: `[]`,
			},
			wantOutput: "Kal'gerion demon",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := buildBossDisplayName(test.event)
			if got != test.wantOutput {
				t.Fatalf("expected %q, got %q", test.wantOutput, got)
			}
		})
	}
}

func TestAssignRanksWithTies(t *testing.T) {
	// This table validates tie handling: entries with the same points
	// share the same rank, and the next rank skips ahead accordingly.
	tests := []struct {
		name           string
		points         []int
		expectedRanks  []int
		expectedPoints []int
	}{
		{
			name:           "unique points get sequential ranks",
			points:         []int{100, 90, 80},
			expectedRanks:  []int{1, 2, 3},
			expectedPoints: []int{100, 90, 80},
		},
		{
			name:           "ties share the same rank",
			points:         []int{100, 100, 90, 80, 80},
			expectedRanks:  []int{1, 1, 3, 4, 4},
			expectedPoints: []int{100, 100, 90, 80, 80},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			entries := make([]LeaderboardEntry, 0, len(test.points))
			for _, pts := range test.points {
				entries = append(entries, LeaderboardEntry{TotalPoints: pts})
			}

			assignRanks(entries)

			for i := range entries {
				if entries[i].TotalPoints != test.expectedPoints[i] {
					t.Fatalf("expected points %d at %d, got %d", test.expectedPoints[i], i, entries[i].TotalPoints)
				}
				if entries[i].CurrentRank != test.expectedRanks[i] {
					t.Fatalf("expected rank %d at %d, got %d", test.expectedRanks[i], i, entries[i].CurrentRank)
				}
			}
		})
	}
}
