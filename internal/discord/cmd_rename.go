package discord

import (
	"context"
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
)

func (b *Bot) renameCommand() Command {
	return Command{
		Definition: &discordgo.ApplicationCommand{
			Name:        "rename",
			Description: "Rename a tracked OSRS account",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:         discordgo.ApplicationCommandOptionString,
					Name:         "current-rsn",
					Description:  "The current RuneScape username",
					Required:     true,
					Autocomplete: true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "new-rsn",
					Description: "The new RuneScape username",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionUser,
					Name:        "user",
					Description: "The user whose account to rename (admin only)",
					Required:    false,
				},
			},
		},
		Handler: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			data := i.ApplicationCommandData()

			if i.GuildID == "" {
				respondError(s, i.Interaction, fmt.Errorf("this command can only be used in a server"))
				return
			}

			var currentRSN, newRSN string
			for _, opt := range data.Options {
				switch opt.Name {
				case "current-rsn":
					currentRSN = opt.StringValue()
				case "new-rsn":
					newRSN = opt.StringValue()
				}
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
				message = fmt.Sprintf("✅ Renaming `%s` to `%s` for <@%s>...", currentRSN, newRSN, user.ID)
			} else {
				message = fmt.Sprintf("✅ Renaming `%s` to `%s`...", currentRSN, newRSN)
			}
			respondSuccess(s, i.Interaction, message)

			// Do heavy work asynchronously (snapshots, leaderboard updates, logging)
			go func() {
				ctx := context.Background()
				if err := b.AccountService.RenameAccount(ctx, targetUser, i.GuildID, currentRSN, newRSN); err != nil {
					log.Printf("Failed to rename account %s to %s: %v", currentRSN, newRSN, err)
					return
				}

				if isOtherUser {
					user, _ := s.User(targetUser)
					b.logAction(ctx, i.GuildID, fmt.Sprintf("✏️ <@%s> renamed account `%s` → `%s` for <@%s>", i.Member.User.ID, currentRSN, newRSN, user.ID))
				} else {
					b.logAction(ctx, i.GuildID, fmt.Sprintf("✏️ <@%s> renamed account `%s` → `%s`", targetUser, currentRSN, newRSN))
				}
			}()
		},
	}
}
