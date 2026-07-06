package services

import (
	"context"
	"fmt"

	"osrs-events/internal/database"
)

type GuildService struct {
	store GuildStore
}

func NewGuildService(store GuildStore) *GuildService {
	return &GuildService{store: store}
}

func (s *GuildService) GetGuild(ctx context.Context, guildID string) (*database.Guild, error) {
	return s.store.GetGuild(ctx, guildID)
}

func (s *GuildService) ListGuildIDs(ctx context.Context) ([]string, error) {
	return s.store.ListGuildIDs(ctx)
}

func (s *GuildService) DeleteGuild(ctx context.Context, guildID string) error {
	return s.store.DeleteGuild(ctx, guildID)
}

func (s *GuildService) PurgeOrphanedEvents(ctx context.Context) (int, error) {
	return s.store.PurgeOrphanedEvents(ctx)
}

func (s *GuildService) GetOrCreateGuild(ctx context.Context, guildID string) (*database.Guild, error) {
	guild, err := s.store.GetGuild(ctx, guildID)
	if err != nil {
		guild = &database.Guild{
			GuildID:      guildID,
			IntervalDay:  "Sunday",
			IntervalTime: "22:00",
		}
		if err := s.store.SaveGuild(ctx, guild); err != nil {
			return nil, fmt.Errorf("failed to create guild: %w", err)
		}
		return guild, nil
	}
	return guild, nil
}

func (s *GuildService) UpdateLogChannel(ctx context.Context, guildID, channelID string) error {
	guild, err := s.GetOrCreateGuild(ctx, guildID)
	if err != nil {
		return err
	}

	guild.LogChannelID = channelID
	return s.store.SaveGuild(ctx, guild)
}

func (s *GuildService) UpdateDonationChannel(ctx context.Context, guildID, channelID string) error {
	guild, err := s.GetOrCreateGuild(ctx, guildID)
	if err != nil {
		return err
	}

	guild.DonationChannelID = channelID
	return s.store.SaveGuild(ctx, guild)
}
