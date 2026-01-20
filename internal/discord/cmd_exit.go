package discord

import (
	"context"
	"fmt"

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
			ctx := context.Background()

			if i.GuildID == "" {
				respondError(s, i.Interaction, fmt.Errorf("this command can only be used in a server"))
				return
			}

			targetUser, isOtherUser, err := getTargetUser(s, i, "user")
			if err != nil {
				respondError(s, i.Interaction, err)
				return
			}

			if err := b.AccountService.ExitCompetition(ctx, targetUser, i.GuildID); err != nil {
				respondError(s, i.Interaction, err)
				return
			}

			var message string
			if isOtherUser {
				user, _ := s.User(targetUser)
				message = fmt.Sprintf("✅ Removed <@%s> from all competitions", user.ID)
			} else {
				message = "✅ You have left all competitions"
			}

			respondSuccess(s, i.Interaction, message)
		},
	}
}
