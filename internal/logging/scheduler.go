package logging

import (
	"log"
	"time"
)

type CleanupScheduler struct {
	cleaner *Cleaner
	ticker  *time.Ticker
	stop    chan struct{}
}

func NewCleanupScheduler(cleaner *Cleaner, interval time.Duration) *CleanupScheduler {
	return &CleanupScheduler{
		cleaner: cleaner,
		ticker:  time.NewTicker(interval),
		stop:    make(chan struct{}),
	}
}

func (s *CleanupScheduler) Start() {
	go func() {
		for {
			select {
			case <-s.ticker.C:
				deleted, err := s.cleaner.Cleanup()
				if err != nil {
					log.Printf("Cleanup error: %v", err)
				} else if deleted > 0 {
					log.Printf("Cleaned up %d old log files", deleted)
				}
			case <-s.stop:
				return
			}
		}
	}()
}

func (s *CleanupScheduler) Stop() {
	s.ticker.Stop()
	close(s.stop)
}
