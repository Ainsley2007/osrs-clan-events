package services

import (
	"strings"
	"testing"

	"osrs-events/internal/database"
)

func TestBuildQuickLinksEmbed_IncludesRulesAndCategoryLists(t *testing.T) {
	groups := []*pbCategoryGroup{
		submissionRulesGroup(),
		{
			Name:  "Minigames",
			Order: 1,
			Categories: []*database.PBCategory{
				{DisplayName: "The Inferno"},
				{DisplayName: "Fight Caves"},
			},
		},
	}
	messageIDs := map[string]string{
		SubmissionRulesGroupName: "msg-rules",
		"Minigames":              "msg-minigames",
	}

	embed := buildQuickLinksEmbed("g1", "ch1", groups, messageIDs)
	if embed.Title != QuickLinksEmbedTitle {
		t.Fatalf("title: got %q", embed.Title)
	}
	if !strings.Contains(embed.Description, "[Submission Rules](https://discord.com/channels/g1/ch1/msg-rules)") {
		t.Fatalf("expected rules jump link, got %q", embed.Description)
	}
	if !strings.Contains(embed.Description, "The Inferno, Fight Caves") {
		t.Fatalf("expected category list for minigames, got %q", embed.Description)
	}
	if strings.Contains(embed.Description, "Submission Rules —") {
		t.Fatalf("rules line should not have category suffix, got %q", embed.Description)
	}
}
