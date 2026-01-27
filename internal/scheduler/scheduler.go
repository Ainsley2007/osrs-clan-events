package scheduler

import (
	"context"
	"log"
	"time"

	"github.com/bwmarrin/discordgo"
)

type Scheduler struct {
	store              Store
	eventService       EventService
	snapshotService    SnapshotService
	leaderboardService LeaderboardService
	session            *discordgo.Session
	clock              Clock
	stopCompletion     chan struct{}
	stopHourly         chan struct{}
}

func New(store Store, eventService EventService, snapshotService SnapshotService, leaderboardService LeaderboardService, session *discordgo.Session) *Scheduler {
	return NewWithClock(store, eventService, snapshotService, leaderboardService, session, realClock{})
}

func NewWithClock(store Store, eventService EventService, snapshotService SnapshotService, leaderboardService LeaderboardService, session *discordgo.Session, clock Clock) *Scheduler {
	return &Scheduler{
		store:              store,
		eventService:       eventService,
		snapshotService:    snapshotService,
		leaderboardService: leaderboardService,
		session:            session,
		clock:              clock,
		stopCompletion:     make(chan struct{}),
		stopHourly:         make(chan struct{}),
	}
}

func (s *Scheduler) Start() {
	log.Println("Starting scheduler...")

	// Process stale events synchronously on startup (events that ended while bot was offline)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	staleEvents, err := s.store.GetStaleEvents(ctx)
	cancel()
	if err != nil {
		log.Printf("Error getting stale events on startup: %v", err)
	} else if len(staleEvents) > 0 {
		log.Printf("Found %d stale events to process on startup", len(staleEvents))
		s.processEventCompletionsForEvents(staleEvents)
	}

	// Take initial snapshot update for all active events asynchronously (don't block startup)
	go func() {
		log.Println("Taking initial snapshot update for active events...")
		s.updateActiveSnapshots()
	}()

	go s.runCompletionCheck()
	go s.runHourlyUpdates()
}

func (s *Scheduler) Stop() {
	log.Println("Stopping scheduler...")
	close(s.stopCompletion)
	close(s.stopHourly)
}
