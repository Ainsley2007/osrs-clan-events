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

			// Respond immediately to avoid timeout
			var message string
			if isOtherUser {
				user, _ := s.User(targetUser)
				message = fmt.Sprintf("✅ Removing <@%s> from all competitions...", user.ID)
			} else {
				message = "✅ Leaving all competitions..."
			}
			respondSuccess(s, i.Interaction, message)

			// Do heavy work asynchronously (leaderboard updates, logging)
			go func() {
				ctx := context.Background()
				if err := b.AccountService.ExitCompetition(ctx, targetUser, i.GuildID); err != nil {
					log.Printf("Failed to exit competition for user %s: %v", targetUser, err)
					return
				}

				if isOtherUser {
					user, _ := s.User(targetUser)
					b.logAction(ctx, i.GuildID, fmt.Sprintf("🚪 <@%s> removed <@%s> from competitions", i.Member.User.ID, user.ID))
				} else {
					b.logAction(ctx, i.GuildID, fmt.Sprintf("🚪 <@%s> left all competitions", targetUser))
				}
			}()
		},
	}
}
