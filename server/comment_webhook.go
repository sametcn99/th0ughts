package server

import (
	"context"
	"log/slog"
	"time"
)

// RunCommentWebhooks dispatches until ctx is cancelled. Store replacement is
// excluded for the duration of each attempt, while HTTP handlers remain concurrent.
func (s *Server) RunCommentWebhooks(ctx context.Context) {
	for ctx.Err() == nil {
		s.mu.RLock()
		wake := s.S.WebhookWake()
		delay, err := s.L.DeliverCommentWebhook(ctx, nil)
		s.mu.RUnlock()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Warn("comment webhook queue temporarily unavailable")
			delay = 10 * time.Second
		}
		if delay <= 0 {
			continue
		}
		if delay > time.Minute {
			delay = time.Minute
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
		case <-wake:
		case <-timer.C:
		}
		timer.Stop()
	}
}
