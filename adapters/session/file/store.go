package file

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"state-machine-engine/internal/logging"
	"strings"
	"sync"
	"time"

	"state-machine-engine/internal/domain"
)

var logger = logging.NewLogger("file/store")

type Store struct {
	mu  sync.RWMutex
	dir string
}

func NewStore(directory string) (*Store, error) {
	logger.Debug("instantiating file session store")

	if directory == "" {
		return nil, errors.New("invalid input: empty session directory")
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create session directory: %w", err)
	}
	return &Store{dir: directory}, nil
}

// Get a session from our file storage.
// Thread-safe and Multiprocess-safe through file locking
func (s *Store) Get(ctx context.Context, sessionID string) (*domain.Session, error) {
	logger.Debug("file store session fetch")

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if sessionID == "" {
		return nil, errors.New("invalid input: empty session id")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	base := s.sessionPath(sessionID)
	unlock, err := acquireFileLock(ctx, base+".lock")
	if err != nil {
		return nil, err
	}
	defer unlock()

	raw, err := os.ReadFile(base)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed reading session file: %w", err)
	}

	var sess domain.Session
	if err = json.Unmarshal(raw, &sess); err != nil {
		return nil, fmt.Errorf("invalid session json: %w", err)
	}

	cp := sess
	if sess.Data != nil {
		cp.Data = json.RawMessage(append([]byte(nil), sess.Data...))
	}
	return &cp, nil
}

// Upsert will insert or update a session into our file storage
// Thread-safe and Multiprocess-safe through file locking
func (s *Store) Upsert(ctx context.Context, session *domain.Session) error {
	logger.Debug("file store session upsert")

	if err := ctx.Err(); err != nil {
		return err
	}
	if session == nil {
		return errors.New("invalid input: nil session")
	}
	if session.ID == "" {
		return errors.New("invalid input: empty session id")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	base := s.sessionPath(session.ID)
	unlock, err := acquireFileLock(ctx, base+".lock")
	if err != nil {
		return err
	}
	defer unlock()

	cp := *session
	if session.Data != nil {
		cp.Data = append([]byte(nil), session.Data...)
	}

	raw, err := json.Marshal(cp)
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	tmp, err := os.CreateTemp(s.dir, "sess-*.tmp")
	if err != nil {
		return fmt.Errorf("failed creating temp session file: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpPath) }
	defer cleanup()

	if _, err = tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("failed writing temp session file: %w", err)
	}
	if err = tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("failed syncing temp session file: %w", err)
	}
	if err = tmp.Close(); err != nil {
		return fmt.Errorf("failed closing temp session file: %w", err)
	}

	if err = os.Rename(tmpPath, base); err != nil {
		return fmt.Errorf("failed replacing session file: %w", err)
	}

	// fsync directory for rename durability.
	dirFD, err := os.Open(s.dir)
	if err != nil {
		return fmt.Errorf("failed opening session directory for sync: %w", err)
	}
	defer func() {
		if err = dirFD.Close(); err != nil {
			logger.Error(fmt.Sprintf("failed closing session directory: %v", err))
		}
	}()

	if err = dirFD.Sync(); err != nil {
		return fmt.Errorf("failed syncing session directory: %w", err)
	}

	return nil
}

// DeleteExpired removes expired sessions and returns how many were deleted.
func (s *Store) DeleteExpired(ctx context.Context) (int, error) {
	logger.Debug("file store delete expired sessions")

	if err := ctx.Err(); err != nil {
		return 0, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return 0, fmt.Errorf("failed reading session directory: %w", err)
	}

	deleted := 0
	now := time.Now()
	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			return deleted, err
		}
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}

		path := filepath.Join(s.dir, e.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			return deleted, fmt.Errorf("failed reading session file %s: %w", e.Name(), err)
		}

		var sess domain.Session
		if err = json.Unmarshal(raw, &sess); err != nil {
			return deleted, fmt.Errorf("invalid session json in %s: %w", e.Name(), err)
		}

		if !sess.ExpiresAt.IsZero() && !sess.ExpiresAt.After(now) {
			if err = os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
				return deleted, fmt.Errorf("failed deleting expired session file %s: %w", e.Name(), err)
			}
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
	logger.Debug("file store session stats")

	if err := ctx.Err(); err != nil {
		return domain.SessionStats{}, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return domain.SessionStats{}, fmt.Errorf("failed reading session directory: %w", err)
	}

	stats := domain.SessionStats{}
	var earliest time.Time
	var latest time.Time
	hasExpiry := false

	now := time.Now()
	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			return stats, err
		}
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}

		path := filepath.Join(s.dir, e.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			return stats, fmt.Errorf("failed reading session file %s: %w", e.Name(), err)
		}

		var sess domain.Session
		if err = json.Unmarshal(raw, &sess); err != nil {
			return stats, fmt.Errorf("invalid session json in %s: %w", e.Name(), err)
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

func (s *Store) sessionPath(sessionID string) string {
	sum := sha256.Sum256([]byte(sessionID))
	name := hex.EncodeToString(sum[:]) + ".json"
	return filepath.Join(s.dir, name)
}

func acquireFileLock(ctx context.Context, lockPath string) (func(), error) {
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			_ = f.Close()
			return func() { _ = os.Remove(lockPath) }, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("failed acquiring file lock: %w", err)
		}

		timer := time.NewTimer(10 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}
