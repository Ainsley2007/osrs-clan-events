package services

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

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

// formatNumber formats an integer with thousand separators (commas)
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

type LeaderboardEntry struct {
	DiscordID    string
	DiscordName  string
	Accounts     []AccountContribution
	AccountCount int // used by overall leaderboard (lightweight, no account data)
	TotalGain    int64
	TotalPoints  int
	CurrentRank  int
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
	description.WriteString(fmt.Sprintf("**%s:** %s\n", thresholdLabel, formatNumber(int64(thresholdValue))))
	if event.Type == "botw" {
		description.WriteString(fmt.Sprintf("**%s:** %g\n", pointsLabel, event.PointsPerKC))
	} else {
		description.WriteString(fmt.Sprintf("**%s:** %g\n", pointsLabel, event.PointsPerXP))
	}
	description.WriteString(fmt.Sprintf("**Time left:** %s\n\n", formatTimeUntil(event.EndTime)))

	if len(entries) == 0 {
		description.WriteString("No participants above threshold yet.")
	} else {
		description.WriteString("**Current Rankings:**\n\n")
		for i, entry := range entries {
			var rankPrefix string
			switch entry.CurrentRank {
			case 1:
				rankPrefix = "🥇"
			case 2:
				rankPrefix = "🥈"
			case 3:
				rankPrefix = "🥉"
			default:
				rankPrefix = fmt.Sprintf("**%d.**", entry.CurrentRank)
			}

			// Count accounts with gain > 0
			accountsWithGain := 0
			for _, acc := range entry.Accounts {
				if acc.Gain > 0 {
					accountsWithGain++
				}
			}

			// Main entry: icon/rank - name - points (optionally with total gain if multiple accounts)
			if accountsWithGain > 1 {
				var gainLabel string
				if event.Type == "botw" {
					gainLabel = "KC"
				} else {
					gainLabel = "XP"
				}
				description.WriteString(fmt.Sprintf("%s <@%s> - points: `%s` (*%s %s*)\n", rankPrefix, entry.DiscordID, formatNumber(int64(entry.TotalPoints)), formatNumber(int64(entry.TotalGain)), gainLabel))
			} else {
				description.WriteString(fmt.Sprintf("%s <@%s> - points: `%s`\n", rankPrefix, entry.DiscordID, formatNumber(int64(entry.TotalPoints))))
			}

			if accountsWithGain >= 1 {
				for _, acc := range entry.Accounts {
					if acc.Gain > 0 {
						var accGainDisplay string
						if event.Type == "botw" {
							accGainDisplay = fmt.Sprintf("%s KC", formatNumber(int64(acc.Gain)))
						} else {
							accGainDisplay = fmt.Sprintf("%s XP", formatNumber(int64(acc.Gain)))
						}
						description.WriteString(fmt.Sprintf("\u2003• *%s: %s*\n", acc.RSN, accGainDisplay))
					}
				}
			}

			if i < len(entries)-1 {
				description.WriteString("\n")
			}
		}
	}

	now := time.Now().UTC()
	return &discordgo.MessageEmbed{
		Title:       title,
		Description: description.String(),
		Color:       color,
		Timestamp:   now.Format(time.RFC3339),
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Last updated",
		},
	}, nil
}

func formatTimeUntil(endTime time.Time) string {
	end := endTime.UTC()
	now := time.Now().UTC()
	if end.Before(now) || end.Equal(now) {
		return "Ended"
	}
	d := end.Sub(now)
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60

	var parts []string
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}
	if hours > 0 || days > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	parts = append(parts, fmt.Sprintf("%dm", mins))
	return strings.Join(parts, " ") + " left"
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
	participants, err := s.store.GetParticipantsByGuild(ctx, guildID)
	if err != nil {
		return nil, fmt.Errorf("failed to get participants: %w", err)
	}

	var entries []LeaderboardEntry
	for _, p := range participants {
		var totalPoints int
		if eventType == "botw" {
			totalPoints = p.TotalPointsBotw
		} else {
			totalPoints = p.TotalPointsSotw
		}
		if totalPoints == 0 {
			continue
		}

		accountCount, _ := s.store.CountActiveAccountsByDiscordID(ctx, p.DiscordUserID)

		user, err := s.session.User(p.DiscordUserID)
		discordName := "Unknown User"
		if err == nil {
			discordName = user.Username
		}

		entries = append(entries, LeaderboardEntry{
			DiscordID:    p.DiscordUserID,
			DiscordName:  discordName,
			TotalPoints:  totalPoints,
			AccountCount: accountCount,
			Accounts:     nil, // not used for overall leaderboard
		})
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

			var rankPrefix string
			switch entry.CurrentRank {
			case 1:
				rankPrefix = "🥇"
			case 2:
				rankPrefix = "🥈"
			case 3:
				rankPrefix = "🥉"
			default:
				rankPrefix = fmt.Sprintf("**%d.**", entry.CurrentRank)
			}

			// Main entry: icon/rank - name - points (optional account count)
			pointsLine := fmt.Sprintf("%s <@%s> - points: `%s`", rankPrefix, entry.DiscordID, formatNumber(int64(entry.TotalPoints)))
			if entry.AccountCount > 0 {
				accLabel := "account"
				if entry.AccountCount != 1 {
					accLabel = "accounts"
				}
				pointsLine += fmt.Sprintf(" (%d %s)", entry.AccountCount, accLabel)
			}
			description.WriteString(pointsLine + "\n")

			// Add spacing between entries (except after the last one)
			if i < len(entries)-1 && i < 19 {
				description.WriteString("\n")
			}
		}

		if len(entries) > 20 {
			description.WriteString(fmt.Sprintf("\n... and %s more", formatNumber(int64(len(entries)-20))))
		}
	}

	return &discordgo.MessageEmbed{
		Title:       title,
		Description: description.String(),
		Color:       color,
	}, nil
}
