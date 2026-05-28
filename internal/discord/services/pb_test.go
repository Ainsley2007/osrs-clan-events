package services

import (
	"strings"
	"testing"
	"time"

	"osrs-events/internal/database"
)

func TestParsePBTimeStrict(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantCS     int64
		wantOutput string
		wantErr    bool
	}{
		{
			name:       "minutes format",
			input:      "59:12.34",
			wantCS:     355234,
			wantOutput: "59:12.34",
		},
		{
			name:       "hours format",
			input:      "1:05:07.08",
			wantCS:     390708,
			wantOutput: "1:05:07.08",
		},
		{
			name:    "invalid no hundredths",
			input:   "12:34",
			wantErr: true,
		},
		{
			name:    "invalid range",
			input:   "61:99.99",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCS, gotOutput, err := ParsePBTimeStrict(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotCS != tt.wantCS {
				t.Fatalf("centiseconds mismatch: got %d, want %d", gotCS, tt.wantCS)
			}
			if gotOutput != tt.wantOutput {
				t.Fatalf("normalized output mismatch: got %q, want %q", gotOutput, tt.wantOutput)
			}
		})
	}
}

func TestBuildLeaderboardEmbed_ShowsTopThreeFastest(t *testing.T) {
	service := &PBService{}
	category := &database.PBCategory{
		Slug:          "inferno",
		DisplayName:   "Inferno",
		EmbedImageURL: "https://oldschool.runescape.wiki/w/Inferno#/media/File:Inferno_logo.png",
	}

	now := time.Now().UTC()
	records := []*database.PBRecord{
		{DisplayName: "Charlie", TimeText: "01:05.50", TimeCentiseconds: 6550, UpdatedAt: now},
		{DisplayName: "Alpha", TimeText: "01:01.00", TimeCentiseconds: 6100, UpdatedAt: now},
		{DisplayName: "Bravo", TimeText: "01:03.20", TimeCentiseconds: 6320, UpdatedAt: now},
		{DisplayName: "Delta", TimeText: "01:10.00", TimeCentiseconds: 7000, UpdatedAt: now},
	}

	embed := service.buildLeaderboardEmbed(category, records[:3])
	if embed == nil {
		t.Fatalf("embed is nil")
	}
	if embed.Image == nil || embed.Image.URL != category.EmbedImageURL {
		t.Fatalf("image url mismatch: got %+v", embed.Image)
	}
	if embed.Footer == nil || embed.Footer.Text != "Last updated" {
		t.Fatalf("expected last updated footer")
	}
	if embed.Timestamp == "" {
		t.Fatalf("expected embed timestamp")
	}

	desc := embed.Description
	if strings.Contains(desc, "<@") {
		t.Fatalf("leaderboard should not include mentions: %q", desc)
	}
	for _, name := range []string{"Alpha", "Bravo", "Charlie"} {
		if !strings.Contains(desc, name) {
			t.Fatalf("expected name %q in leaderboard: %q", name, desc)
		}
	}
	if !strings.Contains(desc, "[proof](") {
		t.Fatalf("expected proof links in leaderboard: %q", desc)
	}
	if !(strings.Index(desc, "Alpha") < strings.Index(desc, "Bravo") &&
		strings.Index(desc, "Bravo") < strings.Index(desc, "Charlie")) {
		t.Fatalf("expected sorted order Alpha -> Bravo -> Charlie, got %q", desc)
	}
}
