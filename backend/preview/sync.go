package preview

import (
	"context"
)

func (s *Service) acquire(ctx context.Context) error {
	select {
	case s.sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Service) release() {
	select {
	case <-s.sem:
	default:
	}
}
