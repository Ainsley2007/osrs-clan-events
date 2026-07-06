package discord

import (
	"strings"
	"testing"

	"osrs-events/internal/discord/services"
)

func TestBuildStatsEmbeds_UsesCodeBlockDescription(t *testing.T) {
	botw := []services.EventStats{{
		WeekNumber: 12,
		MetricName: "Zulrah",
		AccountStats: []services.AccountStat{
			{RSN: "mcainlsey", Gain: 1234},
		},
		Points: 12,
	}}
	sotw := []services.EventStats{{
		WeekNumber: 12,
		MetricName: "Crafting",
		AccountStats: []services.AccountStat{
			{RSN: "mcainlsey", Gain: 50000},
		},
		Points: 5,
	}}

	embeds := buildStatsEmbeds(botw, sotw)
	if len(embeds) != 2 {
		t.Fatalf("expected 2 embeds, got %d", len(embeds))
	}
	for i, embed := range embeds {
		if len(embed.Fields) != 0 {
			t.Fatalf("embed %d: expected no fields, got %d", i, len(embed.Fields))
		}
		if !strings.HasPrefix(embed.Description, "```\n") || !strings.HasSuffix(embed.Description, "```") {
			t.Fatalf("embed %d: expected code block description, got %q", i, embed.Description)
		}
	}
	if !strings.Contains(embeds[0].Description, "Week 12 — Zulrah") {
		t.Fatalf("expected week header in BOTW block:\n%s", embeds[0].Description)
	}
	if !strings.Contains(embeds[0].Description, "mcainlsey") {
		t.Fatalf("expected account line in BOTW block:\n%s", embeds[0].Description)
	}
}

func TestBuildStatsEmbeds_StaysWithinEmbedLimit(t *testing.T) {
	botw := make([]services.EventStats, 12)
	sotw := make([]services.EventStats, 12)
	for i := range botw {
		botw[i] = services.EventStats{
			WeekNumber: i + 1,
			MetricName: "Boss",
			AccountStats: []services.AccountStat{
				{RSN: "Player", Gain: 100},
			},
			Points: 10,
		}
		sotw[i] = botw[i]
		sotw[i].MetricName = "Skill"
	}

	embeds := buildStatsEmbeds(botw, sotw)
	if len(embeds) > maxStatsEmbedsPerMessage {
		t.Fatalf("expected at most %d embeds, got %d", maxStatsEmbedsPerMessage, len(embeds))
	}
	if len(embeds) != 2 {
		t.Fatalf("expected 2 embeds for typical history, got %d", len(embeds))
	}
}

func TestBuildTypeStatsEmbedChunks_SplitsLongHistory(t *testing.T) {
	stats := make([]services.EventStats, 80)
	for i := range stats {
		stats[i] = services.EventStats{
			WeekNumber: i + 1,
			MetricName: "Very Long Boss Name For Padding",
			AccountStats: []services.AccountStat{
				{RSN: "LongPlayerNameHere", Gain: 999999},
				{RSN: "AnotherAccount", Gain: 888888},
			},
			Points: 99,
		}
	}

	embeds := buildTypeStatsEmbedChunks("BOTW progress", stats, "KC", 0)
	if len(embeds) < 2 {
		t.Fatalf("expected multiple chunks for long history, got %d", len(embeds))
	}
	for i, embed := range embeds {
		if len(embed.Description) > 4096 {
			t.Fatalf("embed %d description exceeds Discord limit: %d", i, len(embed.Description))
		}
		if !strings.Contains(embed.Title, "(") {
			t.Fatalf("embed %d expected paginated title, got %q", i, embed.Title)
		}
	}
}

func TestFormatWeekSection(t *testing.T) {
	section := formatWeekSection(services.EventStats{
		WeekNumber: 5,
		MetricName: "Vorkath",
		AccountStats: []services.AccountStat{
			{RSN: "alt", Gain: 42},
		},
		Points: 3,
	}, "KC")

	if !strings.Contains(section, "Week 5 — Vorkath  (3 pts)") {
		t.Fatalf("unexpected section header: %q", section)
	}
	if !strings.Contains(section, "alt") || !strings.Contains(section, "42") {
		t.Fatalf("unexpected account line: %q", section)
	}
}
