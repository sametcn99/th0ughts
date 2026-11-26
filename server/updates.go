package server

import (
	"context"

	"github.com/sametcn99/th0ughts/feed"
	"github.com/sametcn99/th0ughts/updates"
)

// EnableManagedUpdates is called by a worker attached to the release launcher before serving.
func (s *Server) EnableManagedUpdates()            { s.managedUpdates = true }
func (s *Server) UpdateRequested() <-chan struct{} { return s.updateRequested }
func (s *Server) installUpdate(ctx context.Context, a feed.Actor, tag string) error {
	if !a.Recent() {
		return feed.Denied
	}
	if !s.managedUpdates {
		return updates.ErrUnsupported
	}
	if err := s.releases.Stage(ctx, tag); err != nil {
		return err
	}
	select {
	case s.updateRequested <- struct{}{}:
	default:
	}
	return nil
}
