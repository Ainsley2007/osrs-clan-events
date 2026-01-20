package scheduler

import (
	"log"
	"time"

	"osrs-events/internal/database"
)

type Scheduler struct {
	Store    database.Store
	Interval time.Duration
	stop     chan struct{}
}

func New(store database.Store, interval time.Duration) *Scheduler {
	return &Scheduler{
		Store:    store,
		Interval: interval,
		stop:     make(chan struct{}),
	}
}

func (s *Scheduler) Start() {
	ticker := time.NewTicker(s.Interval)
	go func() {
		for {
			select {
			case <-ticker.C:
				s.updateUserData()
			case <-s.stop:
				ticker.Stop()
				return
			}
		}
	}()
}

func (s *Scheduler) Stop() {
	close(s.stop)
}

func (s *Scheduler) updateUserData() {
	log.Println("Starting periodic user data update...")
	// Logic to fetch users from DB, call external API, and update DB
	// users, err := s.Store.GetUsers(context.Background()) ...

	log.Println("Periodic update completed.")
}
