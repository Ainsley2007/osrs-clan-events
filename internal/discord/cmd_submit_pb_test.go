package discord

import (
	"testing"

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
