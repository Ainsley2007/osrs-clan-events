package services

import (
	"context"
	"sync"
	"time"
)

const (
	pbLeaderboardRefreshDebounce = 2 * time.Second
	pbModerationFollowUpTimeout  = 2 * time.Minute
)

type guildWorkDebouncer struct {
	mu      sync.Mutex
	delay   time.Duration
	pending map[string]*time.Timer
	work    func(guildID string)
}

func newGuildWorkDebouncer(delay time.Duration, work func(guildID string)) *guildWorkDebouncer {
	return &guildWorkDebouncer{
		delay:   delay,
		pending: make(map[string]*time.Timer),
		work:    work,
	}
}

func (d *guildWorkDebouncer) schedule(guildID string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if timer, ok := d.pending[guildID]; ok {
		timer.Stop()
	}

	d.pending[guildID] = time.AfterFunc(d.delay, func() {
		d.mu.Lock()
		delete(d.pending, guildID)
		d.mu.Unlock()
		d.work(guildID)
	})
}

func (s *PBService) RunApprovalFollowUp(channelID string, result *PBModerationResult, reviewerDiscordID string, reviewedAt time.Time) {
	if result == nil || result.Submission == nil {
		return
	}
	guildID := result.Submission.GuildID

	go func() {
		if err := s.MarkProofSubmissionAccepted(channelID, result, reviewerDiscordID, reviewedAt); err != nil && s.logger != nil {
			s.logger.Printf("failed to mark accepted pb proof message for submission %d: %v", result.Submission.ID, err)
		}
		s.leaderboardRefreshDebouncer.schedule(guildID)
	}()
}

func (s *PBService) guildLeaderboardRefreshLock(guildID string) *sync.Mutex {
	s.moderationMu.Lock()
	defer s.moderationMu.Unlock()

	if s.guildRefreshLocks == nil {
		s.guildRefreshLocks = make(map[string]*sync.Mutex)
	}
	if s.guildRefreshLocks[guildID] == nil {
		s.guildRefreshLocks[guildID] = &sync.Mutex{}
	}
	return s.guildRefreshLocks[guildID]
}

func (s *PBService) runDebouncedGuildLeaderboardRefresh(guildID string) {
	lock := s.guildLeaderboardRefreshLock(guildID)
	lock.Lock()
	defer lock.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), pbModerationFollowUpTimeout)
	defer cancel()

	if err := s.RefreshAllGroupBundles(ctx, guildID); err != nil && s.logger != nil {
		s.logger.Printf("debounced pb leaderboard refresh failed for guild %s: %v", guildID, err)
	}
}
