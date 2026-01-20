package services

import (
	"context"
	"fmt"
	"log"

	"osrs-events/internal/database"

	"github.com/bwmarrin/discordgo"
)

type InitializerService struct {
	session *discordgo.Session
	store   database.Store
}

func NewInitializerService(session *discordgo.Session, store database.Store) *InitializerService {
	return &InitializerService{
		session: session,
		store:   store,
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
	}

	if err := s.ensureCategories(ctx, guild); err != nil {
		return fmt.Errorf("failed to ensure categories: %w", err)
	}

	if err := s.ensureChannels(guild); err != nil {
		return fmt.Errorf("failed to ensure channels: %w", err)
	}

	if err := s.ensureMessages(guild); err != nil {
		return fmt.Errorf("failed to ensure messages: %w", err)
	}

	if err := s.store.SaveGuild(ctx, guild); err != nil {
		return fmt.Errorf("failed to save guild: %w", err)
	}

	log.Printf("[Guild %s] Initialization complete", guildID)
	return nil
}

func (s *InitializerService) ensureCategories(ctx context.Context, guild *database.Guild) error {
	if err := s.ensureCategory(guild.GuildID, "╔═══BOTW═══╗", &guild.BotwCategoryID); err != nil {
		return err
	}
	if err := s.ensureCategory(guild.GuildID, "╔═══SOTW═══╗", &guild.SotwCategoryID); err != nil {
		return err
	}
	return nil
}

func (s *InitializerService) ensureCategory(guildID, name string, categoryID *string) error {
	if *categoryID != "" {
		_, err := s.session.Channel(*categoryID)
		if err == nil {
			log.Printf("[Guild %s] Category %s exists", guildID, name)
			return nil
		}
		log.Printf("[Guild %s] Category %s not found, recreating", guildID, name)
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

func (s *InitializerService) ensureChannels(guild *database.Guild) error {
	if err := s.ensureChannel(guild.GuildID, "botw-weekly", guild.BotwCategoryID, &guild.BotwChannelID); err != nil {
		return err
	}
	if err := s.ensureChannel(guild.GuildID, "botw-overall", guild.BotwCategoryID, &guild.BotwOverallChannelID); err != nil {
		return err
	}
	if err := s.ensureChannel(guild.GuildID, "sotw-weekly", guild.SotwCategoryID, &guild.SotwChannelID); err != nil {
		return err
	}
	if err := s.ensureChannel(guild.GuildID, "sotw-overall", guild.SotwCategoryID, &guild.SotwOverallChannelID); err != nil {
		return err
	}
	return nil
}

func (s *InitializerService) ensureChannel(guildID, name, parentID string, channelID *string) error {
	if *channelID != "" {
		_, err := s.session.Channel(*channelID)
		if err == nil {
			log.Printf("[Guild %s] Channel %s exists", guildID, name)
			return nil
		}
		log.Printf("[Guild %s] Channel %s not found, recreating", guildID, name)
	}

	channel, err := s.session.GuildChannelCreateComplex(guildID, discordgo.GuildChannelCreateData{
		Name:     name,
		Type:     discordgo.ChannelTypeGuildText,
		ParentID: parentID,
	})
	if err != nil {
		return fmt.Errorf("failed to create channel %s: %w", name, err)
	}

	*channelID = channel.ID
	log.Printf("[Guild %s] Created channel %s: %s", guildID, name, channel.ID)
	return nil
}

func (s *InitializerService) ensureMessages(guild *database.Guild) error {
	if err := s.ensureMessage(guild.GuildID, "BOTW Weekly", guild.BotwChannelID, &guild.BotwMsgID); err != nil {
		return err
	}
	if err := s.ensureMessage(guild.GuildID, "BOTW Overall", guild.BotwOverallChannelID, &guild.BotwOverallMsgID); err != nil {
		return err
	}
	if err := s.ensureMessage(guild.GuildID, "SOTW Weekly", guild.SotwChannelID, &guild.SotwMsgID); err != nil {
		return err
	}
	if err := s.ensureMessage(guild.GuildID, "SOTW Overall", guild.SotwOverallChannelID, &guild.SotwOverallMsgID); err != nil {
		return err
	}
	return nil
}

func (s *InitializerService) ensureMessage(guildID, dashboardType, channelID string, messageID *string) error {
	if *messageID != "" {
		_, err := s.session.ChannelMessage(channelID, *messageID)
		if err == nil {
			log.Printf("[Guild %s] Message for %s exists", guildID, dashboardType)
			return nil
		}
		log.Printf("[Guild %s] Message for %s not found, recreating", guildID, dashboardType)
	}

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
	return nil
}
