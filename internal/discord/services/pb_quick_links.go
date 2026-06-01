package services

import (
	"context"
	"fmt"
	"time"

	"osrs-events/internal/database"

	"github.com/bwmarrin/discordgo"
)

const (
	QuickLinksEmbedTitle         = "Quick Links"
	QuickLinksStateGroupName     = "__quick_links__"
	quickLinksHiddenFieldName    = "\u200b"
)

func discordMessageJumpURL(guildID, channelID, messageID string) string {
	return fmt.Sprintf("https://discord.com/channels/%s/%s/%s", guildID, channelID, messageID)
}

func quickLinksGroupTitleLink(groupName, guildID, channelID, messageID string) string {
	return fmt.Sprintf("[%s](%s)", groupName, discordMessageJumpURL(guildID, channelID, messageID))
}

func buildQuickLinksFieldValue(groupName, guildID, channelID, messageID string, categories []*database.PBCategory) string {
	value := quickLinksGroupTitleLink(groupName, guildID, channelID, messageID)
	for _, category := range categories {
		if category == nil {
			continue
		}
		value += "\n• " + category.DisplayName
	}
	return value
}

func buildQuickLinksEmbed(guildID, channelID string, groups []*pbCategoryGroup, messageIDs map[string]string) *discordgo.MessageEmbed {
	fields := make([]*discordgo.MessageEmbedField, 0, len(groups))
	for _, group := range groups {
		if group == nil {
			continue
		}
		messageID := messageIDs[group.Name]
		if messageID == "" {
			continue
		}

		var categories []*database.PBCategory
		if !group.RulesOnly {
			categories = group.Categories
		}

		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   quickLinksHiddenFieldName,
			Value:  buildQuickLinksFieldValue(group.Name, guildID, channelID, messageID, categories),
			Inline: false,
		})
	}

	return &discordgo.MessageEmbed{
		Title:  QuickLinksEmbedTitle,
		Color:  0x64748B,
		Fields: fields,
	}
}

func (s *PBService) syncQuickLinks(ctx context.Context, guildID, channelID string, groups []*pbCategoryGroup, messageIDs map[string]string) error {
	embed := buildQuickLinksEmbed(guildID, channelID, groups, messageIDs)
	if len(embed.Fields) == 0 {
		return fmt.Errorf("quick links embed has no fields")
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
