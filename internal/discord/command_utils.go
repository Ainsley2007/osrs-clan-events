package discord

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

const cmdTimeout = 30 * time.Second
const guildInitTimeout = 3 * time.Minute

func cmdContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), cmdTimeout)
}

func guildInitContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), guildInitTimeout)
}

func interactionActor(i *discordgo.InteractionCreate) (*discordgo.User, bool) {
	if i == nil {
		return nil, false
	}
	if i.Member != nil && i.Member.User != nil {
		return i.Member.User, true
	}
	if i.User != nil {
		return i.User, true
	}
	return nil, false
}

func interactionActorID(i *discordgo.InteractionCreate) (string, bool) {
	user, ok := interactionActor(i)
	if !ok || user.ID == "" {
		return "", false
	}
	return user.ID, true
}

func ptr[T any](v T) *T { return &v }

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
	commandUser, ok := interactionActorID(i)
	if !ok {
		return "", false, fmt.Errorf("could not resolve command user")
	}

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

func formatNumber(n int64) string {
	s := strconv.FormatInt(n, 10)
	if len(s) <= 3 {
		return s
	}
	var result strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result.WriteRune(',')
		}
		result.WriteRune(r)
	}
	return result.String()
}

// formatAmountM formats a donation/fund amount (actual GP) with "m" suffix, e.g. 1500000 → "1.5m", 100000000 → "100m"
func formatAmountM(gp int64) string {
	millions := float64(gp) / 1_000_000
	return fmt.Sprintf("%gm", millions)
}
