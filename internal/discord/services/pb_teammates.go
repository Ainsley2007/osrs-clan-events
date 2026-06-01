package services

import (
	"fmt"
	"regexp"
	"strings"

	"osrs-events/internal/database"

	"github.com/bwmarrin/discordgo"
)

const MaxPBTeammates = 4

var discordMentionPattern = regexp.MustCompile(`<@!?(\d+)>`)

func parseDiscordMentionIDs(raw string) []string {
	matches := discordMentionPattern.FindAllStringSubmatch(raw, -1)
	ids := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) >= 2 {
			ids = append(ids, match[1])
		}
	}
	return ids
}

func guildMemberDisplayName(member *discordgo.Member) string {
	if member == nil {
		return "Unknown"
	}
	if strings.TrimSpace(member.Nick) != "" {
		return member.Nick
	}
	if member.User != nil {
		if strings.TrimSpace(member.User.GlobalName) != "" {
			return member.User.GlobalName
		}
		return member.User.Username
	}
	return "Unknown"
}

func resolvePBTeammates(session *discordgo.Session, guildID, submitterID, raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	ids := parseDiscordMentionIDs(raw)
	if len(ids) == 0 {
		return nil, fmt.Errorf("no valid teammate mentions found; use @ to tag clan members")
	}
	if len(ids) > MaxPBTeammates {
		return nil, fmt.Errorf("maximum %d teammates allowed", MaxPBTeammates)
	}

	seen := map[string]struct{}{submitterID: {}}
	names := make([]string, 0, len(ids))
	for _, id := range ids {
		if _, dup := seen[id]; dup {
			return nil, fmt.Errorf("duplicate teammate mention")
		}
		seen[id] = struct{}{}

		member, err := session.GuildMember(guildID, id)
		if err != nil {
			return nil, fmt.Errorf("teammate <@%s> is not a member of this server", id)
		}
		names = append(names, guildMemberDisplayName(member))
	}
	return names, nil
}

func FormatLeaderboardDisplayName(submitter string, teammates []string) string {
	submitter = strings.TrimSpace(submitter)
	if len(teammates) == 0 {
		return submitter
	}
	parts := make([]string, 0, 1+len(teammates))
	parts = append(parts, submitter)
	for _, name := range teammates {
		if trimmed := strings.TrimSpace(name); trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return strings.Join(parts, ", ")
}

func submissionLeaderboardDisplayName(submission *database.PBSubmission) string {
	if submission == nil {
		return ""
	}
	if strings.TrimSpace(submission.LeaderboardDisplayName) != "" {
		return submission.LeaderboardDisplayName
	}
	return submission.DisplayName
}
