package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"osrs-events/internal/database"

	"github.com/bwmarrin/discordgo"
)

const (
	QuickLinksEmbedTitle     = "Quick Links"
	QuickLinksStateGroupName = "__quick_links__"
)

func discordMessageJumpURL(guildID, channelID, messageID string) string {
	return fmt.Sprintf("https://discord.com/channels/%s/%s/%s", guildID, channelID, messageID)
}

func buildQuickLinksEmbed(guildID, channelID string, groups []*pbCategoryGroup, messageIDs map[string]string) *discordgo.MessageEmbed {
	var description strings.Builder
	for _, group := range groups {
		if group == nil {
			continue
		}
		messageID := messageIDs[group.Name]
		if messageID == "" {
			continue
		}
		link := discordMessageJumpURL(guildID, channelID, messageID)
		description.WriteString(fmt.Sprintf("[%s](%s)", group.Name, link))
		if !group.RulesOnly {
			names := make([]string, 0, len(group.Categories))
			for _, category := range group.Categories {
				if category == nil {
					continue
				}
				names = append(names, category.DisplayName)
			}
			if len(names) > 0 {
				description.WriteString(" — ")
				description.WriteString(strings.Join(names, ", "))
			}
		}
		description.WriteString("\n")
	}

	return &discordgo.MessageEmbed{
		Title:       QuickLinksEmbedTitle,
		Description: strings.TrimSpace(description.String()),
		Color:       0x64748B,
	}
}

func (s *PBService) syncQuickLinks(ctx context.Context, guildID, channelID string, groups []*pbCategoryGroup, messageIDs map[string]string) error {
	embed := buildQuickLinksEmbed(guildID, channelID, groups, messageIDs)
	if strings.TrimSpace(embed.Description) == "" {
		return fmt.Errorf("quick links embed has no content")
	}

	now := time.Now().UTC()
	state, err := s.store.GetPBGroupBundleMessage(ctx, guildID, QuickLinksStateGroupName)
	if err != nil || state == nil || state.MessageID == "" {
		msg, err := s.session.ChannelMessageSendEmbed(channelID, embed)
		if err != nil {
			return fmt.Errorf("failed to post quick links message: %w", err)
		}
		return s.store.UpsertPBGroupBundleMessage(ctx, &database.PBLeaderboardMessage{
			GuildID:   guildID,
			GroupName: QuickLinksStateGroupName,
			ChannelID: channelID,
			MessageID: msg.ID,
			UpdatedAt: now,
		})
	}

	if _, err := s.session.ChannelMessage(state.ChannelID, state.MessageID); err != nil {
		return fmt.Errorf("quick links message missing: %w", err)
	}

	embeds := []*discordgo.MessageEmbed{embed}
	if _, err := s.session.ChannelMessageEditComplex(&discordgo.MessageEdit{
		ID:      state.MessageID,
		Channel: state.ChannelID,
		Embeds:  &embeds,
	}); err != nil {
		return fmt.Errorf("failed to update quick links message: %w", err)
	}

	state.UpdatedAt = now
	return s.store.UpsertPBGroupBundleMessage(ctx, state)
}

func groupMessageIDsFromStates(states []*database.PBLeaderboardMessage) map[string]string {
	ids := make(map[string]string)
	for _, state := range states {
		if state == nil || state.GroupName == QuickLinksStateGroupName || state.MessageID == "" {
			continue
		}
		ids[state.GroupName] = state.MessageID
	}
	return ids
}
