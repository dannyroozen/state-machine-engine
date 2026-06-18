// Simple in-memory session storage mechanism provided as an example.
// Other implementations could include elasticache or DynamoDB, for example.

package memory

import (
	"context"
	"encoding/json"
	"errors"
	"sync"

	"state-machine-engine/internal/domain"
)

type Store struct {
	mu       sync.RWMutex
	sessions map[string]domain.Session
}

func NewStore() *Store {
	return &Store{
		sessions: make(map[string]domain.Session),
	}
}

func (s *Store) Get(_ context.Context, sessionID string) (*domain.Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	val, ok := s.sessions[sessionID]
	if !ok {
		return nil, nil
	}

	cp := val
	return &cp, nil
}

func (s *Store) Upsert(_ context.Context, session *domain.Session) error {
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
