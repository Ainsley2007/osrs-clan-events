package discord

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func hasAdminPermission(s *discordgo.Session, guildID, userID string) bool {
	guild, err := s.Guild(guildID)
	if err != nil {
		return false
	}

	if guild.OwnerID == userID {
		return true
	}

	member, err := s.GuildMember(guildID, userID)
	if err != nil {
		return false
	}

	for _, roleID := range member.Roles {
		role, err := s.State.Role(guildID, roleID)
		if err != nil {
			continue
		}
		if (role.Permissions & discordgo.PermissionAdministrator) == discordgo.PermissionAdministrator {
			return true
		}
	}

	return false
}

func parseUserMention(mention string) (string, error) {
	mention = strings.TrimSpace(mention)

	if strings.HasPrefix(mention, "<@") && strings.HasSuffix(mention, ">") {
		mention = strings.TrimPrefix(mention, "<@")
		mention = strings.TrimPrefix(mention, "!")
		mention = strings.TrimSuffix(mention, ">")
		return mention, nil
	}

	return "", fmt.Errorf("invalid user mention format")
}

func getTargetUser(s *discordgo.Session, i *discordgo.InteractionCreate, optionName string) (string, bool, error) {
	data := i.ApplicationCommandData()
	commandUser := i.Member.User.ID

	for _, opt := range data.Options {
		if opt.Name == optionName {
			targetUser := opt.UserValue(s).ID

			if targetUser != commandUser {
				if !hasAdminPermission(s, i.GuildID, commandUser) {
					return "", false, fmt.Errorf("you don't have permission to manage other users' accounts")
				}
				return targetUser, true, nil
			}
			return targetUser, false, nil
		}
	}

	return commandUser, false, nil
}
