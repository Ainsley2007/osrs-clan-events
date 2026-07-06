package scheduler

import (
	"context"
	"log"
	"sync"
	"time"
)

type Scheduler struct {
	store              Store
	eventService       EventService
	snapshotService    SnapshotService
	leaderboardService LeaderboardService
	initializerService InitializerService
	notifier           Notifier
	clock              Clock
	missingAccountMu   sync.Mutex
	stopCompletion     chan struct{}
	stopHourly         chan struct{}
}

func New(store Store, eventService EventService, snapshotService SnapshotService, leaderboardService LeaderboardService, initializerService InitializerService, notifier Notifier) *Scheduler {
	return NewWithClock(store, eventService, snapshotService, leaderboardService, initializerService, notifier, realClock{})
}

func NewWithClock(store Store, eventService EventService, snapshotService SnapshotService, leaderboardService LeaderboardService, initializerService InitializerService, notifier Notifier, clock Clock) *Scheduler {
	if notifier == nil {
		notifier = noopNotifier{}
	}
	return &Scheduler{
		store:              store,
		eventService:       eventService,
		snapshotService:    snapshotService,
		leaderboardService: leaderboardService,
		initializerService: initializerService,
		notifier:           notifier,
		clock:              clock,
		stopCompletion:     make(chan struct{}),
		stopHourly:         make(chan struct{}),
	}
}

func (s *Scheduler) Start() {
	log.Println("Starting scheduler...")

	// Process expired active events synchronously on startup (events that ended while bot was offline)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	expiredEvents, err := s.store.GetExpiredActiveEvents(ctx)
	cancel()
	if err != nil {
		log.Printf("Error getting expired active events on startup: %v", err)
	} else if len(expiredEvents) > 0 {
		log.Printf("Found %d expired active events to process on startup", len(expiredEvents))
		s.processEventCompletionsForEvents(expiredEvents)
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
