// Simple in-memory session storage mechanism provided as an example.
// Other implementations could include elasticache or DynamoDB, for example.

package memory

import (
	"context"
	"encoding/json"
	"errors"
	"state-machine-engine/internal/logging"
	"sync"
	"time"

	"state-machine-engine/internal/domain"
)

var logger = logging.NewLogger("memory/store")

type Store struct {
	mu       sync.RWMutex
	sessions map[string]domain.Session
}

func NewStore() *Store {
	logger.Debug("instantiating memory session store")
	return &Store{
		sessions: make(map[string]domain.Session),
	}
}

// Get a session from our in-memory storage.
// Thread-safe
func (s *Store) Get(_ context.Context, sessionID string) (*domain.Session, error) {
	logger.Debug("memory store session fetch")

	s.mu.RLock()
	defer s.mu.RUnlock()

	val, ok := s.sessions[sessionID]
	if !ok {
		return nil, nil
	}

	cp := val
	return &cp, nil
}

// Upsert will insert or update a session into our in-memory storage
// Thread-safe
func (s *Store) Upsert(_ context.Context, session *domain.Session) error {
	logger.Debug("memory store session upsert")
	if session == nil {
		return errors.New("invalid input: nil session")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	cp := *session
	if session.Data != nil {
		cp.Data = json.RawMessage(append([]byte(nil), session.Data...))
	}
	s.sessions[session.ID] = cp
	return nil
}

// DeleteExpired removes all expired sessions and returns the number deleted.
func (s *Store) DeleteExpired(ctx context.Context) (int, error) {
	logger.Debug("memory store delete expired sessions")

	if err := ctx.Err(); err != nil {
		return 0, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	deleted := 0
	now := time.Now()
	for id, sess := range s.sessions {
		if err := ctx.Err(); err != nil {
			return deleted, err
		}
		if !sess.ExpiresAt.IsZero() && !sess.ExpiresAt.After(now) {
			delete(s.sessions, id)
			deleted++
		}
	}
	return deleted, nil
}

// ActiveCount returns the number of sessions that are not expired at the provided timestamp.
func (s *Store) ActiveCount(ctx context.Context) (int, error) {
	stats, err := s.Stats(ctx)
	if err != nil {
		return 0, err
	}
	return stats.Active, nil
}

func (s *Store) Stats(ctx context.Context) (domain.SessionStats, error) {
	logger.Debug("memory store session stats")

	if err := ctx.Err(); err != nil {
		return domain.SessionStats{}, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := domain.SessionStats{}
	var earliest time.Time
	var latest time.Time
	hasExpiry := false

	now := time.Now()
	for _, sess := range s.sessions {
		if err := ctx.Err(); err != nil {
			return stats, err
		}
		stats.Total++
		if sess.ExpiresAt.IsZero() {
			stats.WithoutExpiry++
			stats.Active++
			continue
		}
		if sess.ExpiresAt.After(now) {
			stats.Active++
		} else {
			stats.Expired++
		}
		if !hasExpiry {
			earliest = sess.ExpiresAt
			latest = sess.ExpiresAt
			hasExpiry = true
		} else {
			if sess.ExpiresAt.Before(earliest) {
				earliest = sess.ExpiresAt
			}
			if sess.ExpiresAt.After(latest) {
				latest = sess.ExpiresAt
			}
		}
	}

	if hasExpiry {
		earliestCopy := earliest
		latestCopy := latest
		stats.EarliestExpiry = &earliestCopy
		stats.LatestExpiry = &latestCopy
	}

	return stats, nil
}
