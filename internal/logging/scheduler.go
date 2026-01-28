package logging

import (
	"log"
	"sync"
	"time"
)

type CleanupScheduler struct {
	cleaner  *Cleaner
	ticker   *time.Ticker
	stop     chan struct{}
	stopOnce sync.Once
}

func NewCleanupScheduler(cleaner *Cleaner, interval time.Duration) *CleanupScheduler {
	return &CleanupScheduler{
		cleaner: cleaner,
		ticker:  time.NewTicker(interval),
		stop:    make(chan struct{}),
	}
}

func (s *CleanupScheduler) Start() {
	// Run initial cleanup immediately
	go s.runCleanup()

	go func() {
		for {
			select {
			case <-s.ticker.C:
				s.runCleanup()
			case <-s.stop:
				return
			}
		}
	}()
}

func (s *CleanupScheduler) runCleanup() {
	deleted, err := s.cleaner.Cleanup()
	if err != nil {
		log.Printf("Cleanup error: %v", err)
	} else if deleted > 0 {
		log.Printf("Cleaned up %d old log files", deleted)
	}
}

func (s *CleanupScheduler) Stop() {
	s.stopOnce.Do(func() {
		s.ticker.Stop()
		close(s.stop)
	})
}
