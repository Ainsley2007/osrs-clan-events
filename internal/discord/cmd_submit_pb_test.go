package discord

import (
	"testing"

	"osrs-events/internal/database"

	"github.com/bwmarrin/discordgo"
)

func TestGetAttachmentOptionValue(t *testing.T) {
	data := discordgo.ApplicationCommandInteractionData{
		Resolved: &discordgo.ApplicationCommandInteractionDataResolved{
			Attachments: map[string]*discordgo.MessageAttachment{
				"att-1": {ID: "att-1", URL: "https://example.com/proof.png"},
			},
		},
	}
	opt := &discordgo.ApplicationCommandInteractionDataOption{
		Name:  "attachment",
		Type:  discordgo.ApplicationCommandOptionAttachment,
		Value: "att-1",
	}

	attachment, err := getAttachmentOptionValue(data, opt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if attachment.URL != "https://example.com/proof.png" {
		t.Fatalf("attachment url mismatch: got %s", attachment.URL)
	}
}

func TestInteractionUserAndDisplayName(t *testing.T) {
	i := &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			Member: &discordgo.Member{
				Nick: "NickName",
				User: &discordgo.User{ID: "u1", Username: "username"},
			},
		},
	}

	userID, displayName := interactionUserAndDisplayName(i)
	if userID != "u1" {
		t.Fatalf("user id mismatch: got %s", userID)
	}
	if displayName != "NickName" {
		t.Fatalf("display name mismatch: got %s", displayName)
	}
}

func TestPBCategoryAutocompleteChoices_AlphabeticalAndPrefixFilter(t *testing.T) {
	categories := []*database.PBCategory{
		{Slug: "yama", DisplayName: "Yama", IsActive: true},
		{Slug: "gauntlet", DisplayName: "Gauntlet", IsActive: true},
		{Slug: "inferno", DisplayName: "The Inferno", IsActive: true},
		{Slug: "inactive", DisplayName: "Inactive", IsActive: false},
	}

	choices := pbCategoryAutocompleteChoices(categories, "")
	if len(choices) != 3 {
		t.Fatalf("expected 3 active categories, got %d", len(choices))
	}
	if choices[0].Name != "Gauntlet" || choices[0].Value != "gauntlet" {
		t.Fatalf("expected first choice Gauntlet, got %#v", choices[0])
	}
	if choices[1].Name != "The Inferno" || choices[2].Name != "Yama" {
		t.Fatalf("unexpected alphabetical order: %#v", choices)
	}

	filtered := pbCategoryAutocompleteChoices(categories, "ga")
	if len(filtered) != 1 {
		t.Fatalf("expected 1 ga-prefix match, got %d", len(filtered))
	}
	if filtered[0].Value != "gauntlet" {
		t.Fatalf("expected gauntlet match, got %#v", filtered[0])
	}
}

func TestPBCategoryAutocompleteQuery_FocusedCategoryOption(t *testing.T) {
	options := []*discordgo.ApplicationCommandInteractionDataOption{
		{Name: "attachment", Type: discordgo.ApplicationCommandOptionAttachment, Focused: false},
		{Name: "category", Type: discordgo.ApplicationCommandOptionString, Focused: true, Value: "duke"},
	}
	if got := pbCategoryAutocompleteQuery(options); got != "duke" {
		t.Fatalf("expected focused category query duke, got %q", got)
	}
}
