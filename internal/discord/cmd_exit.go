package discord

import (
	"context"
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
)

func (b *Bot) exitCommand() Command {
	return Command{
		Definition: &discordgo.ApplicationCommand{
			Name:        "exit",
			Description: "Leave all competitions and stop tracking your accounts",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionUser,
					Name:        "user",
					Description: "The user to remove from competitions (admin only)",
					Required:    false,
				},
			},
		},
		Handler: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			if i.GuildID == "" {
				respondError(s, i.Interaction, fmt.Errorf("this command can only be used in a server"))
				return
			}

			targetUser, isOtherUser, err := getTargetUser(s, i, "user")
			if err != nil {
				respondError(s, i.Interaction, err)
				return
			}

			if err := respondDeferred(s, i.Interaction, "⏳ Leaving competitions..."); err != nil {
				log.Printf("Failed to defer exit interaction: %v", err)
				return
			}

			go func() {
				ctx := context.Background()
				if err := b.AccountService.ExitCompetition(ctx, targetUser, i.GuildID); err != nil {
					editDeferredContent(s, i.Interaction, fmt.Sprintf("❌ Failed to leave competitions: %v", err))
					return
				}

				if isOtherUser {
					user, _ := s.User(targetUser)
					b.logAction(ctx, i.GuildID, fmt.Sprintf("🚪 <@%s> removed <@%s> from competitions", i.Member.User.ID, user.ID))
					editDeferredContent(s, i.Interaction, fmt.Sprintf("✅ Removed <@%s> from all competitions.", targetUser))
				} else {
					b.logAction(ctx, i.GuildID, fmt.Sprintf("🚪 <@%s> left all competitions", targetUser))
					editDeferredContent(s, i.Interaction, "✅ Left all competitions.")
				}
			}()
		},
	}
}
