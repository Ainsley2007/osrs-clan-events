package services

import (
	"context"
	"fmt"
	"log"
	"strings"

	"osrs-events/internal/database"

	"github.com/bwmarrin/discordgo"
)

type InitializerService struct {
	session            *discordgo.Session
	store              database.Store
	leaderboardService *LeaderboardService
}

func NewInitializerService(session *discordgo.Session, store database.Store, leaderboardService *LeaderboardService) *InitializerService {
	return &InitializerService{
		session:            session,
		store:              store,
		leaderboardService: leaderboardService,
	}
}

func (s *InitializerService) InitializeGuild(ctx context.Context, guildID string) error {
	log.Printf("[Guild %s] Starting initialization...", guildID)

	guild, err := s.store.GetGuild(ctx, guildID)
	if err != nil {
		guild = &database.Guild{
			GuildID:      guildID,
			IntervalDay:  "Sunday",
			IntervalTime: "22:00",
		}
		log.Printf("[Guild %s] Creating new guild entry", guildID)
		if err := s.store.SaveGuild(ctx, guild); err != nil {
			return fmt.Errorf("failed to create guild entry: %w", err)
		}
	}

	if err := s.ensureCategories(ctx, guild); err != nil {
		return fmt.Errorf("failed to ensure categories: %w", err)
	}

	if err := s.ensureChannels(ctx, guild); err != nil {
		return fmt.Errorf("failed to ensure channels: %w", err)
	}

	if err := s.ensureMessages(ctx, guild); err != nil {
		return fmt.Errorf("failed to ensure messages: %w", err)
	}

	// Refresh all leaderboards after ensuring messages exist
	if err := s.refreshLeaderboards(ctx, guildID); err != nil {
		log.Printf("[Guild %s] Failed to refresh leaderboards: %v", guildID, err)
		// Don't fail initialization if leaderboard refresh fails
	}

	log.Printf("[Guild %s] Initialization complete", guildID)
	return nil
}

// categoryDisplayName returns the display name for a BOTW/SOTW category.
// When metricName is empty, returns base name (e.g. "╔═══BOTW═══╗").
// When metricName is set, returns event-specific name (e.g. "╔═══BOTW - Vorkath═══╗").
func categoryDisplayName(eventType, metricName string) string {
	upper := strings.ToUpper(eventType)
	if metricName == "" {
		return fmt.Sprintf("╔═══%s═══╗", upper)
	}
	return fmt.Sprintf("╔═══%s - %s═══╗", upper, metricName)
}

func (s *InitializerService) ensureCategories(ctx context.Context, guild *database.Guild) error {
	botwMetric := ""
	if e, err := s.store.GetActiveEvent(ctx, guild.GuildID, "botw"); err == nil && e != nil {
		botwMetric = e.MetricJsonID
	}
	if err := s.ensureCategory(ctx, guild.GuildID, "botw", botwMetric, &guild.BotwCategoryID); err != nil {
		return err
	}
	if err := s.store.SaveGuild(ctx, guild); err != nil {
		return fmt.Errorf("failed to save guild after BOTW category: %w", err)
	}

	sotwMetric := ""
	if e, err := s.store.GetActiveEvent(ctx, guild.GuildID, "sotw"); err == nil && e != nil {
		sotwMetric = e.MetricJsonID
	}
	if err := s.ensureCategory(ctx, guild.GuildID, "sotw", sotwMetric, &guild.SotwCategoryID); err != nil {
		return err
	}
	if err := s.store.SaveGuild(ctx, guild); err != nil {
		return fmt.Errorf("failed to save guild after SOTW category: %w", err)
	}

	return nil
}

func (s *InitializerService) ensureCategory(_ context.Context, guildID, eventType, metricName string, categoryID *string) error {
	name := categoryDisplayName(eventType, metricName)
	if *categoryID != "" {
		channel, err := s.session.Channel(*categoryID)
		if err == nil && channel != nil {
			log.Printf("[Guild %s] Category %s exists", guildID, name)
			return nil
		}
		log.Printf("[Guild %s] Category %s (ID: %s) not found in Discord, recreating", guildID, name, *categoryID)
	}

	channel, err := s.session.GuildChannelCreateComplex(guildID, discordgo.GuildChannelCreateData{
		Name: name,
		Type: discordgo.ChannelTypeGuildCategory,
	})
	if err != nil {
		return fmt.Errorf("failed to create category %s: %w", name, err)
	}

	*categoryID = channel.ID
	log.Printf("[Guild %s] Created category %s: %s", guildID, name, channel.ID)
	return nil
}

// RenameCategory renames a Discord category channel
func (s *InitializerService) RenameCategory(ctx context.Context, guildID, categoryID, newName string) error {
	if categoryID == "" {
		return fmt.Errorf("category ID is empty")
	}

	_, err := s.session.ChannelEdit(categoryID, &discordgo.ChannelEdit{
		Name: newName,
	})
	if err != nil {
		return fmt.Errorf("failed to rename category %s to %s: %w", categoryID, newName, err)
	}

	log.Printf("[Guild %s] Renamed category %s to %s", guildID, categoryID, newName)
	return nil
}

// RenameCategoryForEvent renames the category based on the event type and metric name
func (s *InitializerService) RenameCategoryForEvent(ctx context.Context, guild *database.Guild, eventType string, event *database.Event) error {
	var categoryID string
	if eventType == "botw" {
		categoryID = guild.BotwCategoryID
	} else {
		categoryID = guild.SotwCategoryID
	}
	if categoryID == "" {
		return nil
	}
	// Use MetricJsonID (boss/skill name), not tracked bosses
	newName := categoryDisplayName(eventType, event.MetricJsonID)
	return s.RenameCategory(ctx, guild.GuildID, categoryID, newName)
}

func (s *InitializerService) ensureChannels(ctx context.Context, guild *database.Guild) error {
	if err := s.ensureChannel(ctx, guild.GuildID, "botw-weekly", guild.BotwCategoryID, &guild.BotwChannelID); err != nil {
		return err
	}
	if err := s.store.SaveGuild(ctx, guild); err != nil {
		return fmt.Errorf("failed to save guild after botw-weekly: %w", err)
	}

	if err := s.ensureChannel(ctx, guild.GuildID, "botw-overall", guild.BotwCategoryID, &guild.BotwOverallChannelID); err != nil {
		return err
	}
	if err := s.store.SaveGuild(ctx, guild); err != nil {
		return fmt.Errorf("failed to save guild after botw-overall: %w", err)
	}

	if err := s.ensureChannel(ctx, guild.GuildID, "sotw-weekly", guild.SotwCategoryID, &guild.SotwChannelID); err != nil {
		return err
	}
	if err := s.store.SaveGuild(ctx, guild); err != nil {
		return fmt.Errorf("failed to save guild after sotw-weekly: %w", err)
	}

	if err := s.ensureChannel(ctx, guild.GuildID, "sotw-overall", guild.SotwCategoryID, &guild.SotwOverallChannelID); err != nil {
		return err
	}
	if err := s.store.SaveGuild(ctx, guild); err != nil {
		return fmt.Errorf("failed to save guild after sotw-overall: %w", err)
	}

	return nil
}

func (s *InitializerService) ensureChannel(_ context.Context, guildID, name, parentID string, channelID *string) error {
	if *channelID != "" {
		channel, err := s.session.Channel(*channelID)
		if err == nil && channel != nil {
			log.Printf("[Guild %s] Channel %s exists", guildID, name)
			return nil
		}
		log.Printf("[Guild %s] Channel %s (ID: %s) not found in Discord, recreating", guildID, name, *channelID)
	}

	// Read-only: @everyone can view but not send; bot, server owner, and admin role can send
	allowSend := int64(discordgo.PermissionViewChannel | discordgo.PermissionSendMessages)
	overwrites := []*discordgo.PermissionOverwrite{
		{ID: guildID, Type: discordgo.PermissionOverwriteTypeRole, Allow: discordgo.PermissionViewChannel, Deny: discordgo.PermissionSendMessages},
	}
	if s.session.State != nil && s.session.State.User != nil {
		overwrites = append(overwrites, &discordgo.PermissionOverwrite{
			ID: s.session.State.User.ID, Type: discordgo.PermissionOverwriteTypeMember, Allow: allowSend,
		})
	}
	guild, err := s.session.Guild(guildID)
	if err == nil {
		if guild.OwnerID != "" {
			overwrites = append(overwrites, &discordgo.PermissionOverwrite{
				ID: guild.OwnerID, Type: discordgo.PermissionOverwriteTypeMember, Allow: allowSend,
			})
		}
		for _, r := range guild.Roles {
			if strings.EqualFold(r.Name, "admin") {
				overwrites = append(overwrites, &discordgo.PermissionOverwrite{
					ID: r.ID, Type: discordgo.PermissionOverwriteTypeRole, Allow: allowSend,
				})
				break
			}
		}
	}

	channel, err := s.session.GuildChannelCreateComplex(guildID, discordgo.GuildChannelCreateData{
		Name:                name,
		Type:                discordgo.ChannelTypeGuildText,
		ParentID:            parentID,
		PermissionOverwrites: overwrites,
	})
	if err != nil {
		return fmt.Errorf("failed to create channel %s: %w", name, err)
	}

	*channelID = channel.ID
	log.Printf("[Guild %s] Created channel %s: %s", guildID, name, channel.ID)
	return nil
}

func (s *InitializerService) ensureMessages(ctx context.Context, guild *database.Guild) error {
	if err := s.ensureMessage(ctx, guild.GuildID, "BOTW Weekly", "botw", "weekly", guild.BotwChannelID, &guild.BotwMsgID); err != nil {
		return err
	}
	if err := s.store.SaveGuild(ctx, guild); err != nil {
		return fmt.Errorf("failed to save guild after BOTW Weekly message: %w", err)
	}

	if err := s.ensureMessage(ctx, guild.GuildID, "BOTW Overall", "botw", "overall", guild.BotwOverallChannelID, &guild.BotwOverallMsgID); err != nil {
		return err
	}
	if err := s.store.SaveGuild(ctx, guild); err != nil {
		return fmt.Errorf("failed to save guild after BOTW Overall message: %w", err)
	}

	if err := s.ensureMessage(ctx, guild.GuildID, "SOTW Weekly", "sotw", "weekly", guild.SotwChannelID, &guild.SotwMsgID); err != nil {
		return err
	}
	if err := s.store.SaveGuild(ctx, guild); err != nil {
		return fmt.Errorf("failed to save guild after SOTW Weekly message: %w", err)
	}

	if err := s.ensureMessage(ctx, guild.GuildID, "SOTW Overall", "sotw", "overall", guild.SotwOverallChannelID, &guild.SotwOverallMsgID); err != nil {
		return err
	}
	if err := s.store.SaveGuild(ctx, guild); err != nil {
		return fmt.Errorf("failed to save guild after SOTW Overall message: %w", err)
	}

	return nil
}

func (s *InitializerService) ensureMessage(ctx context.Context, guildID, dashboardType, eventType, leaderboardType string, channelID string, messageID *string) error {
	if *messageID != "" {
		msg, err := s.session.ChannelMessage(channelID, *messageID)
		if err == nil && msg != nil {
			log.Printf("[Guild %s] Message for %s exists", guildID, dashboardType)
			return nil
		}
		log.Printf("[Guild %s] Message for %s (ID: %s) not found in Discord, recreating", guildID, dashboardType, *messageID)
		*messageID = "" // Clear invalid message ID
	}

	// If message doesn't exist, create a placeholder first
	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("%s Dashboard", dashboardType),
		Description: "Dashboard will be updated when competition starts.",
		Color:       0x5865F2,
	}

	msg, err := s.session.ChannelMessageSendEmbed(channelID, embed)
	if err != nil {
		return fmt.Errorf("failed to create message for %s: %w", dashboardType, err)
	}

	*messageID = msg.ID
	log.Printf("[Guild %s] Created message for %s: %s", guildID, dashboardType, msg.ID)

	// Now try to update with actual leaderboard data if available
	if leaderboardType == "weekly" {
		if err := s.leaderboardService.UpdateWeeklyLeaderboard(ctx, guildID, eventType); err == nil {
			log.Printf("[Guild %s] Updated %s leaderboard with active event data", guildID, dashboardType)
		}
	} else {
		// Overall leaderboard - always try to update (doesn't require active event)
		if err := s.leaderboardService.UpdateOverallLeaderboard(ctx, guildID, eventType); err == nil {
			log.Printf("[Guild %s] Updated %s leaderboard", guildID, dashboardType)
		}
	}

	return nil
}

func (s *InitializerService) refreshLeaderboards(ctx context.Context, guildID string) error {
	// Update weekly leaderboards if active events exist
	if err := s.leaderboardService.UpdateWeeklyLeaderboard(ctx, guildID, "botw"); err != nil {
		// Log but don't fail - event might not exist
		log.Printf("[Guild %s] Could not update BOTW weekly leaderboard: %v", guildID, err)
	}
	if err := s.leaderboardService.UpdateWeeklyLeaderboard(ctx, guildID, "sotw"); err != nil {
		log.Printf("[Guild %s] Could not update SOTW weekly leaderboard: %v", guildID, err)
	}

	// Always update overall leaderboards (they don't require active events)
	if err := s.leaderboardService.UpdateOverallLeaderboard(ctx, guildID, "botw"); err != nil {
		log.Printf("[Guild %s] Could not update BOTW overall leaderboard: %v", guildID, err)
	}
	if err := s.leaderboardService.UpdateOverallLeaderboard(ctx, guildID, "sotw"); err != nil {
		log.Printf("[Guild %s] Could not update SOTW overall leaderboard: %v", guildID, err)
	}

	return nil
}
