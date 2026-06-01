package services

import (
	"strings"
	"testing"

	"osrs-events/internal/database"
)

func TestBuildQuickLinksEmbed_ClickableGroupTitlesAndBullets(t *testing.T) {
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
	if len(embed.Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(embed.Fields))
	}

	rules := embed.Fields[0]
	if rules.Name != quickLinksHiddenFieldName {
		t.Fatalf("expected hidden field name, got %q", rules.Name)
	}
	rulesLink := "[Submission Rules](https://discord.com/channels/g1/ch1/msg-rules)"
	if rules.Value != rulesLink {
		t.Fatalf("rules value: got %q want %q", rules.Value, rulesLink)
	}
	if strings.Contains(rules.Value, "View section") {
		t.Fatalf("rules should not include view section label, got %q", rules.Value)
	}

	minigames := embed.Fields[1]
	if !strings.HasPrefix(minigames.Value, "[Minigames](https://discord.com/channels/g1/ch1/msg-minigames)") {
		t.Fatalf("expected clickable group title, got %q", minigames.Value)
	}
	if !strings.Contains(minigames.Value, "• The Inferno") || !strings.Contains(minigames.Value, "• Fight Caves") {
		t.Fatalf("expected bullet categories, got %q", minigames.Value)
	}
}
