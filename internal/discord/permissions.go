package discord

import (
	"errors"

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
		if role.Permissions&discordgo.PermissionAdministrator != 0 {
			return true
		}
	}

	return false
}

func requireGuildActor(s *discordgo.Session, i *discordgo.InteractionCreate) (*discordgo.User, bool) {
	actor, ok := interactionActor(i)
	if !ok {
		respondError(s, i.Interaction, errors.New("could not resolve command user"))
		return nil, false
	}
	return actor, true
}

func requireAdmin(s *discordgo.Session, i *discordgo.InteractionCreate) (*discordgo.User, bool) {
	actor, ok := requireGuildActor(s, i)
	if !ok {
		return nil, false
	}
	if !hasAdminPermission(s, i.GuildID, actor.ID) {
		respondError(s, i.Interaction, errors.New("you must be an administrator to use this command"))
		return nil, false
	}
	return actor, true
}
