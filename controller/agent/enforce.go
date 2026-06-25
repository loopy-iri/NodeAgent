package agent

import (
	"context"
	"log"
	"time"
)

// RunEnforcement periodically collects per-user traffic and applies quota and
// expiry enforcement until the context is cancelled.
func (s *Server) RunEnforcement(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 10 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.mgr.CollectAndEnforce(ctx, time.Now().Unix()); err != nil {
				log.Printf("enforcement cycle error: %v", err)
			}
		}
	}
}
