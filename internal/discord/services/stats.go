package services

import (
	"context"
	"fmt"
	"sort"

	"osrs-events/internal/database"
)

type StatsService struct {
	store StatsStore
}

type StatsStore interface {
	GetAccountsByDiscordID(ctx context.Context, discordUserID string) ([]*database.Account, error)
	GetAllEventsByGuildAndType(ctx context.Context, guildID string, eventType string) ([]*database.Event, error)
	GetSnapshot(ctx context.Context, eventID, accountID int64) (*database.Snapshot, error)
}

func NewStatsService(store StatsStore) *StatsService {
	return &StatsService{
		store: store,
	}
}

type EventStats struct {
	EventID      int64
	EventType    string
	WeekNumber   int
	MetricName   string
	AccountStats []AccountStat
	TotalGain    int64
	Points       int
}

type AccountStat struct {
	RSN  string
	Gain int64
}

func (s *StatsService) GetUserEventStats(ctx context.Context, discordUserID, guildID string) ([]EventStats, []EventStats, error) {
	// Get all accounts for this user
	accounts, err := s.store.GetAccountsByDiscordID(ctx, discordUserID)
	if err != nil || len(accounts) == 0 {
		return nil, nil, fmt.Errorf("no tracked accounts found")
	}

	// Get all BOTW events for this guild
	botwEvents, err := s.store.GetAllEventsByGuildAndType(ctx, guildID, "botw")
	if err != nil {
		botwEvents = []*database.Event{}
	}

	// Get all SOTW events for this guild
	sotwEvents, err := s.store.GetAllEventsByGuildAndType(ctx, guildID, "sotw")
	if err != nil {
		sotwEvents = []*database.Event{}
	}

	// Build BOTW stats
	var botwStats []EventStats
	for _, event := range botwEvents {
		stats := s.getEventStats(ctx, event, accounts)
		if len(stats) > 0 {
			totalGain := int64(0)
			for _, stat := range stats {
				totalGain += stat.Gain
			}
			points := s.calculatePoints(event, totalGain)
			
			botwStats = append(botwStats, EventStats{
				EventID:      event.ID,
				EventType:    event.Type,
				WeekNumber:   event.WeekNumber,
				MetricName:   event.MetricJsonID,
				AccountStats: stats,
				TotalGain:    totalGain,
				Points:       points,
			})
		}
	}

	// Build SOTW stats
	var sotwStats []EventStats
	for _, event := range sotwEvents {
		stats := s.getEventStats(ctx, event, accounts)
		if len(stats) > 0 {
			totalGain := int64(0)
			for _, stat := range stats {
				totalGain += stat.Gain
			}
			points := s.calculatePoints(event, totalGain)
			
			sotwStats = append(sotwStats, EventStats{
				EventID:      event.ID,
				EventType:    event.Type,
				WeekNumber:   event.WeekNumber,
				MetricName:   event.MetricJsonID,
				AccountStats: stats,
				TotalGain:    totalGain,
				Points:       points,
			})
		}
	}

	return botwStats, sotwStats, nil
}

func (s *StatsService) getEventStats(ctx context.Context, event *database.Event, accounts []*database.Account) []AccountStat {
	var stats []AccountStat

	for _, account := range accounts {
		snapshot, err := s.store.GetSnapshot(ctx, event.ID, account.ID)
		if err != nil {
			continue // No snapshot for this account
		}

		gain := snapshot.CurrentValue - snapshot.StartValue
		if gain > 0 {
			stats = append(stats, AccountStat{
				RSN:  account.RSN,
				Gain: gain,
			})
		}
	}

	// Sort by gain descending
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].Gain > stats[j].Gain
	})

	return stats
}

func (s *StatsService) calculatePoints(event *database.Event, totalGain int64) int {
	// Check threshold
	var threshold int64
	if event.Type == "botw" {
		threshold = int64(event.ThresholdKC)
	} else {
		threshold = int64(event.XPThreshold)
	}

	if totalGain < threshold {
		return 0
	}

	// Calculate points
	var points int
	if event.Type == "botw" {
		points = int(float64(totalGain) * event.PointsPerKC)
	} else {
		points = int(float64(totalGain) * event.PointsPerXP)
	}

	return points
}
