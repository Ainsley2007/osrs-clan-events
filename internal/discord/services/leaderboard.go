package services

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"osrs-events/internal/database"

	"github.com/bwmarrin/discordgo"
)

type LeaderboardService struct {
	store   LeaderboardStore
	session *discordgo.Session
	logger  Logger
}

func NewLeaderboardService(store LeaderboardStore, session *discordgo.Session, logger Logger) *LeaderboardService {
	return &LeaderboardService{
		store:   store,
		session: session,
		logger:  logger,
	}
}

type LeaderboardEntry struct {
	DiscordID   string
	DiscordName string
	Accounts    []AccountContribution
	TotalGain   int64
	TotalPoints int
	CurrentRank int
}

type AccountContribution struct {
	RSN  string
	Gain int64
}

func (s *LeaderboardService) UpdateWeeklyLeaderboard(ctx context.Context, guildID string, eventType string) error {
	guild, err := s.store.GetGuild(ctx, guildID)
	if err != nil {
		return fmt.Errorf("failed to get guild: %w", err)
	}

	event, err := s.store.GetActiveEvent(ctx, guildID, eventType)
	if err != nil {
		return fmt.Errorf("no active event found: %w", err)
	}

	var channelID, messageID string
	if eventType == "botw" {
		channelID = guild.BotwChannelID
		messageID = guild.BotwMsgID
	} else {
		channelID = guild.SotwChannelID
		messageID = guild.SotwMsgID
	}

	if channelID == "" || messageID == "" {
		return fmt.Errorf("channel or message ID not set for %s", eventType)
	}

	embed, err := s.buildWeeklyLeaderboardEmbed(ctx, event)
	if err != nil {
		return fmt.Errorf("failed to build leaderboard embed: %w", err)
	}

	_, err = s.session.ChannelMessageEditEmbed(channelID, messageID, embed)
	if err != nil {
		return fmt.Errorf("failed to update leaderboard message: %w", err)
	}

	return nil
}

func (s *LeaderboardService) RefreshLeaderboards(ctx context.Context, guildID string) {
	if err := s.UpdateWeeklyLeaderboard(ctx, guildID, "botw"); err != nil {
		s.logger.Printf("Failed to update BOTW weekly leaderboard: %v", err)
	}
	if err := s.UpdateWeeklyLeaderboard(ctx, guildID, "sotw"); err != nil {
		s.logger.Printf("Failed to update SOTW weekly leaderboard: %v", err)
	}
	if err := s.UpdateOverallLeaderboard(ctx, guildID, "botw"); err != nil {
		s.logger.Printf("Failed to update BOTW overall leaderboard: %v", err)
	}
	if err := s.UpdateOverallLeaderboard(ctx, guildID, "sotw"); err != nil {
		s.logger.Printf("Failed to update SOTW overall leaderboard: %v", err)
	}
}

func (s *LeaderboardService) UpdateOverallLeaderboard(ctx context.Context, guildID string, eventType string) error {
	guild, err := s.store.GetGuild(ctx, guildID)
	if err != nil {
		return fmt.Errorf("failed to get guild: %w", err)
	}

	var channelID, messageID string
	if eventType == "botw" {
		channelID = guild.BotwOverallChannelID
		messageID = guild.BotwOverallMsgID
	} else {
		channelID = guild.SotwOverallChannelID
		messageID = guild.SotwOverallMsgID
	}

	if channelID == "" || messageID == "" {
		return fmt.Errorf("channel or message ID not set for %s overall", eventType)
	}

	embed, err := s.buildOverallLeaderboardEmbed(ctx, guildID, eventType)
	if err != nil {
		return fmt.Errorf("failed to build overall leaderboard embed: %w", err)
	}

	_, err = s.session.ChannelMessageEditEmbed(channelID, messageID, embed)
	if err != nil {
		return fmt.Errorf("failed to update overall leaderboard message: %w", err)
	}

	return nil
}

func (s *LeaderboardService) buildWeeklyLeaderboardEmbed(ctx context.Context, event *database.Event) (*discordgo.MessageEmbed, error) {
	snapshotsWithAccounts, err := s.store.GetSnapshotsWithAccounts(ctx, event.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshots: %w", err)
	}

	// Group by Discord user ID and combine gains
	userMap := make(map[string]*LeaderboardEntry)
	for _, swa := range snapshotsWithAccounts {
		gain := swa.Snapshot.CurrentValue - swa.Snapshot.StartValue
		if gain < 0 {
			gain = 0
		}

		discordID := swa.Account.DiscordUserID
		if _, exists := userMap[discordID]; !exists {
			userMap[discordID] = &LeaderboardEntry{
				DiscordID: discordID,
				Accounts:  []AccountContribution{},
			}
		}

		entry := userMap[discordID]
		entry.Accounts = append(entry.Accounts, AccountContribution{
			RSN:  swa.Account.RSN,
			Gain: gain,
		})
		entry.TotalGain += gain
	}

	// Calculate points and filter by threshold
	var entries []LeaderboardEntry
	for _, entry := range userMap {
		// Filter by threshold (combined gain must meet threshold)
		if event.Type == "botw" {
			if entry.TotalGain < int64(event.ThresholdKC) {
				continue
			}
		} else {
			if entry.TotalGain < int64(event.XPThreshold) {
				continue
			}
		}

		// Calculate total points
		if event.Type == "botw" {
			entry.TotalPoints = int(float64(entry.TotalGain) * event.PointsPerKC)
		} else {
			entry.TotalPoints = int(float64(entry.TotalGain) * event.PointsPerXP)
		}

		// Fetch Discord username
		user, err := s.session.User(entry.DiscordID)
		if err != nil {
			entry.DiscordName = "Unknown User"
		} else {
			entry.DiscordName = user.Username
		}

		entries = append(entries, *entry)
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].TotalPoints != entries[j].TotalPoints {
			return entries[i].TotalPoints > entries[j].TotalPoints
		}
		// If points are equal, sort by Discord name alphabetically for consistency
		return entries[i].DiscordName < entries[j].DiscordName
	})

	assignRanks(entries)

	var title, metricLabel, pointsLabel, thresholdLabel string
	var color int
	var thresholdValue int

	var bossDisplayName string
	if event.Type == "botw" {
		title = "🎯 Boss of the Week - Weekly Leaderboard"
		metricLabel = "Boss"
		pointsLabel = "Points/KC"
		thresholdLabel = "Threshold KC"
		color = 0x00FF00
		thresholdValue = event.ThresholdKC

		bossDisplayName = buildBossDisplayName(event)
	} else {
		title = "📚 Skill of the Week - Weekly Leaderboard"
		metricLabel = "Skill"
		pointsLabel = "Points/XP"
		thresholdLabel = "XP Threshold"
		color = 0x0099FF
		thresholdValue = event.XPThreshold
	}

	var description strings.Builder
	if event.Type == "botw" {
		description.WriteString(fmt.Sprintf("**%s:** %s\n", metricLabel, bossDisplayName))
	} else {
		description.WriteString(fmt.Sprintf("**%s:** %s\n", metricLabel, event.MetricJsonID))
	}
	description.WriteString(fmt.Sprintf("**%s:** %d\n", thresholdLabel, thresholdValue))
	if event.Type == "botw" {
		description.WriteString(fmt.Sprintf("**%s:** %.2f\n\n", pointsLabel, event.PointsPerKC))
	} else {
		description.WriteString(fmt.Sprintf("**%s:** %.2f\n\n", pointsLabel, event.PointsPerXP))
	}

	if len(entries) == 0 {
		description.WriteString("No participants above threshold yet.")
	} else {
		description.WriteString("**Current Rankings:**\n")
		for _, entry := range entries {
			medal := ""
			switch entry.CurrentRank {
			case 1:
				medal = "🥇 "
			case 2:
				medal = "🥈 "
			case 3:
				medal = "🥉 "
			}

			var gainDisplay string
			if event.Type == "botw" {
				gainDisplay = fmt.Sprintf("%d KC", entry.TotalGain)
			} else {
				gainDisplay = fmt.Sprintf("%d XP", entry.TotalGain)
			}

			description.WriteString(fmt.Sprintf("%s**%d.** <@%s> - **%d pts** (%s)\n",
				medal, entry.CurrentRank, entry.DiscordID, entry.TotalPoints, gainDisplay))

			// Show account breakdown
			for _, acc := range entry.Accounts {
				if acc.Gain > 0 {
					var accGainDisplay string
					if event.Type == "botw" {
						accGainDisplay = fmt.Sprintf("%d KC", acc.Gain)
					} else {
						accGainDisplay = fmt.Sprintf("%d XP", acc.Gain)
					}
					description.WriteString(fmt.Sprintf("   └─ %s: %s\n", acc.RSN, accGainDisplay))
				}
			}
		}
	}

	return &discordgo.MessageEmbed{
		Title:       title,
		Description: description.String(),
		Color:       color,
	}, nil
}

func buildBossDisplayName(event *database.Event) string {
	var bossesToTrack []string
	if err := json.Unmarshal([]byte(event.BossesToTrack), &bossesToTrack); err != nil || len(bossesToTrack) == 0 {
		return event.MetricJsonID
	}
	if len(bossesToTrack) == 1 {
		return bossesToTrack[0]
	}
	return strings.Join(bossesToTrack, " + ")
}

func assignRanks(entries []LeaderboardEntry) {
	currentRank := 1
	for i := range entries {
		if i > 0 && entries[i].TotalPoints != entries[i-1].TotalPoints {
			currentRank = i + 1
		}
		entries[i].CurrentRank = currentRank
	}
}

func (s *LeaderboardService) buildOverallLeaderboardEmbed(ctx context.Context, guildID string, eventType string) (*discordgo.MessageEmbed, error) {
	accounts, err := s.store.GetAccountsByGuild(ctx, guildID)
	if err != nil {
		return nil, fmt.Errorf("failed to get accounts: %w", err)
	}

	// Group by Discord user ID
	userMap := make(map[string]*LeaderboardEntry)
	for _, account := range accounts {
		participant, err := s.store.GetParticipant(ctx, account.DiscordUserID, guildID)
		if err != nil || participant == nil {
			continue
		}

		var totalPoints int
		if eventType == "botw" {
			totalPoints = participant.TotalPointsBotw
		} else {
			totalPoints = participant.TotalPointsSotw
		}

		if totalPoints == 0 {
			continue
		}

		discordID := account.DiscordUserID
		if _, exists := userMap[discordID]; !exists {
			userMap[discordID] = &LeaderboardEntry{
				DiscordID:   discordID,
				TotalPoints: totalPoints,
				Accounts:    []AccountContribution{},
			}
		}

		entry := userMap[discordID]
		entry.Accounts = append(entry.Accounts, AccountContribution{
			RSN: account.RSN,
		})
		// Use the highest total points (they should all be the same for the same user)
		if totalPoints > entry.TotalPoints {
			entry.TotalPoints = totalPoints
		}
	}

	var entries []LeaderboardEntry
	for _, entry := range userMap {
		// Fetch Discord username
		user, err := s.session.User(entry.DiscordID)
		if err != nil {
			entry.DiscordName = "Unknown User"
		} else {
			entry.DiscordName = user.Username
		}

		entries = append(entries, *entry)
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].TotalPoints != entries[j].TotalPoints {
			return entries[i].TotalPoints > entries[j].TotalPoints
		}
		// If points are equal, sort by Discord name alphabetically for consistency
		return entries[i].DiscordName < entries[j].DiscordName
	})

	assignRanks(entries)

	var title string
	var color int
	if eventType == "botw" {
		title = "🎯 Boss of the Week - Overall Leaderboard"
		color = 0x00FF00
	} else {
		title = "📚 Skill of the Week - Overall Leaderboard"
		color = 0x0099FF
	}

	var description strings.Builder
	description.WriteString("**All-Time Rankings**\n\n")

	if len(entries) == 0 {
		description.WriteString("No participants yet.")
	} else {
		for i, entry := range entries {
			if i >= 20 {
				break
			}

			medal := ""
			switch entry.CurrentRank {
			case 1:
				medal = "🥇 "
			case 2:
				medal = "🥈 "
			case 3:
				medal = "🥉 "
			}

			description.WriteString(fmt.Sprintf("%s**%d.** <@%s> - **%d pts**\n",
				medal, entry.CurrentRank, entry.DiscordID, entry.TotalPoints))

			// Show account breakdown
			if len(entry.Accounts) > 0 {
				accountNames := make([]string, 0, len(entry.Accounts))
				for _, acc := range entry.Accounts {
					accountNames = append(accountNames, acc.RSN)
				}
				description.WriteString(fmt.Sprintf("   └─ Accounts: %s\n", strings.Join(accountNames, ", ")))
			}
		}

		if len(entries) > 20 {
			description.WriteString(fmt.Sprintf("\n... and %d more", len(entries)-20))
		}
	}

	return &discordgo.MessageEmbed{
		Title:       title,
		Description: description.String(),
		Color:       color,
	}, nil
}
