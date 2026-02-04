package services

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"osrs-events/internal/database"

	"github.com/bwmarrin/discordgo"
)

type DonationService struct {
	store   DonationStore
	session *discordgo.Session
	logger  Logger
}

func NewDonationService(store DonationStore, session *discordgo.Session, logger Logger) *DonationService {
	return &DonationService{
		store:   store,
		session: session,
		logger:  logger,
	}
}

type DonationLeaderboardEntry struct {
	DiscordID   string
	DiscordName string
	TotalAmount int64
	CurrentRank int
}

type FundsSummary struct {
	TotalDonated int64
	TotalSpent   int64
	Available    int64
}

func (s *DonationService) AddDonation(ctx context.Context, guildID, userID string, amount int, createdBy string) error {
	if amount <= 0 {
		return fmt.Errorf("donation amount must be positive")
	}

	donation := &database.Donation{
		GuildID:       guildID,
		DiscordUserID: userID,
		Amount:        int64(amount),
		CreatedAt:     time.Now().UTC(),
		CreatedBy:     createdBy,
	}

	return s.store.SaveDonation(ctx, donation)
}

func (s *DonationService) UseFunds(ctx context.Context, guildID string, amount int, description string, createdBy string) error {
	if amount <= 0 {
		return fmt.Errorf("amount must be positive")
	}

	// Check available funds
	summary, err := s.GetFundsSummary(ctx, guildID)
	if err != nil {
		return fmt.Errorf("failed to get funds summary: %w", err)
	}

	if summary.Available < int64(amount) {
		return fmt.Errorf("insufficient funds: available %s, requested %s", formatNumber(summary.Available), formatNumber(int64(amount)))
	}

	spending := &database.DonationSpending{
		GuildID:     guildID,
		Amount:      int64(amount),
		Description: description,
		CreatedAt:   time.Now().UTC(),
		CreatedBy:   createdBy,
	}

	return s.store.SaveDonationSpending(ctx, spending)
}

func (s *DonationService) GetFundsSummary(ctx context.Context, guildID string) (*FundsSummary, error) {
	donations, err := s.store.GetDonationsByGuild(ctx, guildID)
	if err != nil {
		return nil, fmt.Errorf("failed to get donations: %w", err)
	}

	var totalDonated int64
	for _, d := range donations {
		totalDonated += d.Amount
	}

	totalSpent, err := s.store.GetTotalSpent(ctx, guildID)
	if err != nil {
		return nil, fmt.Errorf("failed to get total spent: %w", err)
	}

	return &FundsSummary{
		TotalDonated: totalDonated,
		TotalSpent:   totalSpent,
		Available:    totalDonated - totalSpent,
	}, nil
}

func (s *DonationService) GetDonationLeaderboard(ctx context.Context, guildID string) ([]DonationLeaderboardEntry, error) {
	donations, err := s.store.GetDonationsByGuild(ctx, guildID)
	if err != nil {
		return nil, fmt.Errorf("failed to get donations: %w", err)
	}

	// Aggregate by user
	userTotals := make(map[string]int64)
	for _, d := range donations {
		userTotals[d.DiscordUserID] += d.Amount
	}

	var entries []DonationLeaderboardEntry
	for userID, total := range userTotals {
		if total <= 0 {
			continue
		}

		user, err := s.session.User(userID)
		discordName := "Unknown User"
		if err == nil {
			discordName = user.Username
		}

		entries = append(entries, DonationLeaderboardEntry{
			DiscordID:   userID,
			DiscordName: discordName,
			TotalAmount: total,
		})
	}

	// Sort by total amount descending
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].TotalAmount != entries[j].TotalAmount {
			return entries[i].TotalAmount > entries[j].TotalAmount
		}
		return entries[i].DiscordName < entries[j].DiscordName
	})

	// Assign ranks
	currentRank := 1
	for i := range entries {
		if i > 0 && entries[i].TotalAmount != entries[i-1].TotalAmount {
			currentRank = i + 1
		}
		entries[i].CurrentRank = currentRank
	}

	return entries, nil
}

func (s *DonationService) CreateOrUpdateLeaderboard(ctx context.Context, guildID string) error {
	guild, err := s.store.GetGuild(ctx, guildID)
	if err != nil {
		return fmt.Errorf("failed to get guild: %w", err)
	}

	if guild.DonationChannelID == "" {
		return fmt.Errorf("donation channel not set")
	}

	embed, err := s.buildDonationLeaderboardEmbed(ctx, guildID)
	if err != nil {
		return fmt.Errorf("failed to build leaderboard embed: %w", err)
	}

	if guild.DonationMsgID == "" {
		// Create new message
		msg, err := s.session.ChannelMessageSendEmbed(guild.DonationChannelID, embed)
		if err != nil {
			return fmt.Errorf("failed to create leaderboard message: %w", err)
		}
		if err := s.store.UpdateDonationMessageID(ctx, guildID, msg.ID); err != nil {
			return fmt.Errorf("failed to save message ID: %w", err)
		}
	} else {
		// Update existing message
		_, err = s.session.ChannelMessageEditEmbed(guild.DonationChannelID, guild.DonationMsgID, embed)
		if err != nil {
			return fmt.Errorf("failed to update leaderboard message: %w", err)
		}
	}

	return nil
}

func (s *DonationService) UpdateLeaderboard(ctx context.Context, guildID string) error {
	return s.CreateOrUpdateLeaderboard(ctx, guildID)
}

func (s *DonationService) buildDonationLeaderboardEmbed(ctx context.Context, guildID string) (*discordgo.MessageEmbed, error) {
	entries, err := s.GetDonationLeaderboard(ctx, guildID)
	if err != nil {
		return nil, err
	}

	summary, err := s.GetFundsSummary(ctx, guildID)
	if err != nil {
		return nil, err
	}

	var description strings.Builder
	description.WriteString("**Summary:**\n")
	description.WriteString(fmt.Sprintf("Total Contributed to Fund: `%s`\n", formatNumber(summary.TotalDonated)))
	description.WriteString(fmt.Sprintf("Total Spent from Fund: `%s`\n", formatNumber(summary.TotalSpent)))
	description.WriteString(fmt.Sprintf("Available Fund: `%s`\n\n", formatNumber(summary.Available)))

	if len(entries) == 0 {
		description.WriteString("No donations yet.")
	} else {
		description.WriteString("**Top Donators:**\n\n")
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

			description.WriteString(fmt.Sprintf("%s <@%s> - `%s`\n", rankPrefix, entry.DiscordID, formatNumber(entry.TotalAmount)))

			if i < len(entries)-1 && i < 19 {
				description.WriteString("\n")
			}
		}

		if len(entries) > 20 {
			description.WriteString(fmt.Sprintf("\n... and %s more", formatNumber(int64(len(entries)-20))))
		}
	}

	now := time.Now().UTC()
	return &discordgo.MessageEmbed{
		Title:       "💰 Donation Leaderboard",
		Description: description.String(),
		Color:       0xFFD700,
		Timestamp:   now.Format(time.RFC3339),
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Last updated",
		},
	}, nil
}
