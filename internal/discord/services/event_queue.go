package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"osrs-events/internal/firebase"
)

var (
	ErrUnknownBoss  = errors.New("unknown boss")
	ErrUnknownSkill = errors.New("unknown skill")
)

func (s *EventService) ListBotwQueue(ctx context.Context, guildID string) ([]string, error) {
	return s.store.ListMetricQueue(ctx, guildID, "botw")
}

func (s *EventService) ListSotwQueue(ctx context.Context, guildID string) ([]string, error) {
	return s.store.ListMetricQueue(ctx, guildID, "sotw")
}

func (s *EventService) AddBotwQueue(ctx context.Context, guildID, bossName string) (string, error) {
	config, err := s.configProvider.FetchOSRSConfig(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to fetch OSRS config: %w", err)
	}
	boss, ok := findBossConfig(config.Bosses, bossName)
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnknownBoss, bossName)
	}
	if err := s.store.AppendMetricQueue(ctx, guildID, "botw", boss.Name); err != nil {
		return "", err
	}
	return boss.Name, nil
}

func (s *EventService) AddSotwQueue(ctx context.Context, guildID, skillName string) (string, error) {
	config, err := s.configProvider.FetchOSRSConfig(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to fetch OSRS config: %w", err)
	}
	skill, ok := findSkillConfig(config.Skills, skillName)
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnknownSkill, skillName)
	}
	if err := s.store.AppendMetricQueue(ctx, guildID, "sotw", skill.Name); err != nil {
		return "", err
	}
	return skill.Name, nil
}

func (s *EventService) RemoveBotwQueueAt(ctx context.Context, guildID string, position int) (string, error) {
	return s.store.RemoveMetricQueueAt(ctx, guildID, "botw", position)
}

func (s *EventService) RemoveSotwQueueAt(ctx context.Context, guildID string, position int) (string, error) {
	return s.store.RemoveMetricQueueAt(ctx, guildID, "sotw", position)
}

func (s *EventService) ClearBotwQueue(ctx context.Context, guildID string) (int, error) {
	return s.store.ClearMetricQueue(ctx, guildID, "botw")
}

func (s *EventService) ClearSotwQueue(ctx context.Context, guildID string) (int, error) {
	return s.store.ClearMetricQueue(ctx, guildID, "sotw")
}

func (s *EventService) BossNamesFromConfig(ctx context.Context) ([]string, error) {
	config, err := s.configProvider.FetchOSRSConfig(ctx)
	if err != nil {
		return nil, err
	}
	names := make([]string, len(config.Bosses))
	for i, b := range config.Bosses {
		names[i] = b.Name
	}
	return names, nil
}

func (s *EventService) SkillNamesFromConfig(ctx context.Context) ([]string, error) {
	config, err := s.configProvider.FetchOSRSConfig(ctx)
	if err != nil {
		return nil, err
	}
	names := make([]string, len(config.Skills))
	for i, sk := range config.Skills {
		names[i] = sk.Name
	}
	return names, nil
}

func findBossConfig(bosses []firebase.BossConfig, name string) (*firebase.BossConfig, bool) {
	trimmed := strings.TrimSpace(name)
	for i := range bosses {
		if strings.EqualFold(bosses[i].Name, trimmed) {
			return &bosses[i], true
		}
	}
	return nil, false
}

func findSkillConfig(skills []firebase.SkillConfig, name string) (*firebase.SkillConfig, bool) {
	trimmed := strings.TrimSpace(name)
	for i := range skills {
		if strings.EqualFold(skills[i].Name, trimmed) {
			return &skills[i], true
		}
	}
	return nil, false
}

func (s *EventService) pickQueuedBossConfig(ctx context.Context, guildID string, bosses []firebase.BossConfig) (*firebase.BossConfig, bool, error) {
	name, ok, err := s.pickQueuedMetricName(ctx, guildID, "botw", func(queued string) bool {
		_, found := findBossConfig(bosses, queued)
		return found
	})
	if err != nil || !ok {
		return nil, ok, err
	}
	boss, _ := findBossConfig(bosses, name)
	return boss, true, nil
}

func (s *EventService) pickQueuedSkillConfig(ctx context.Context, guildID string, skills []firebase.SkillConfig) (*firebase.SkillConfig, bool, error) {
	name, ok, err := s.pickQueuedMetricName(ctx, guildID, "sotw", func(queued string) bool {
		_, found := findSkillConfig(skills, queued)
		return found
	})
	if err != nil || !ok {
		return nil, ok, err
	}
	skill, _ := findSkillConfig(skills, name)
	return skill, true, nil
}

func (s *EventService) pickQueuedMetricName(ctx context.Context, guildID, eventType string, valid func(string) bool) (string, bool, error) {
	for {
		queued, err := s.store.PeekMetricQueue(ctx, guildID, eventType)
		if err != nil {
			return "", false, err
		}
		if queued == "" {
			return "", false, nil
		}
		if valid(queued) {
			return queued, true, nil
		}
		if _, err := s.store.PopMetricQueue(ctx, guildID, eventType); err != nil {
			return "", false, err
		}
		if s.logger != nil {
			s.logger.Printf("[Guild %s] %s queue: removed stale entry %q (not in remote config)", guildID, strings.ToUpper(eventType), queued)
		}
	}
}

func (s *EventService) consumeQueueHeadIfMatches(ctx context.Context, guildID, eventType, metricName string) {
	head, err := s.store.PeekMetricQueue(ctx, guildID, eventType)
	if err != nil || head == "" {
		return
	}
	if !strings.EqualFold(head, metricName) {
		return
	}
	if _, err := s.store.PopMetricQueue(ctx, guildID, eventType); err != nil && s.logger != nil {
		s.logger.Printf("[Guild %s] %s queue: failed to consume head %q: %v", guildID, strings.ToUpper(eventType), head, err)
	}
}
