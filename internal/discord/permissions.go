package discord

import (
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
		// Check if the user has a role named "admin" (case-insensitive)
		if strings.EqualFold(role.Name, "admin") {
			return true
		}
	}

	return false
}
