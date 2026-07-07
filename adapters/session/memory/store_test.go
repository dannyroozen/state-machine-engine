package memory

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"state-machine-engine/internal/domain"
)

func TestStore_GetMissing(t *testing.T) {
	t.Parallel()

	s := NewStore()
	got, err := s.Get(context.Background(), "missing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
}

func TestStore_UpsertAndGet_CopySemantics(t *testing.T) {
	t.Parallel()

	s := NewStore()
	orig := &domain.Session{
		ID:        "s1",
		State:     "start",
		Data:      json.RawMessage(`{"a":1}`),
		ExpiresAt: time.Now().Add(time.Minute),
	}
	if err := s.Upsert(context.Background(), orig); err != nil {
		t.Fatalf("unexpected upsert error: %v", err)
	}

	got, err := s.Get(context.Background(), "s1")
	if err != nil {
		t.Fatalf("unexpected get error: %v", err)
	}
	if got == nil || got.ID != "s1" || got.State != "start" {
		t.Fatalf("unexpected session: %+v", got)
	}

	orig.State = "mutated"
	orig.Data[0] = '{'
	got2, _ := s.Get(context.Background(), "s1")
	if got2.State != "start" {
		t.Fatalf("store should keep copy, got state %q", got2.State)
	}
}

func TestStore_UpsertNil(t *testing.T) {
	t.Parallel()

	s := NewStore()
	err := s.Upsert(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "invalid input: nil session") {
		t.Fatalf("expected nil session error, got %v", err)
	}
}

func TestStore_DeleteExpiredAndActiveCount(t *testing.T) {
	t.Parallel()

	s := NewStore()
	now := time.Now()

	_ = s.Upsert(context.Background(), &domain.Session{ID: "active", State: "x", ExpiresAt: now.Add(time.Minute)})
	_ = s.Upsert(context.Background(), &domain.Session{ID: "expired", State: "x", ExpiresAt: now.Add(-time.Minute)})
	_ = s.Upsert(context.Background(), &domain.Session{ID: "no-expiry", State: "x"})

	activeBefore, err := s.ActiveCount(context.Background())
	if err != nil {
		t.Fatalf("unexpected active count error: %v", err)
	}
	if activeBefore != 2 {
		t.Fatalf("expected 2 active sessions, got %d", activeBefore)
	}

	deleted, err := s.DeleteExpired(context.Background())
	if err != nil {
		t.Fatalf("unexpected delete expired error: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("expected 1 deleted session, got %d", deleted)
	}

	activeAfter, err := s.ActiveCount(context.Background())
	if err != nil {
		t.Fatalf("unexpected active count error: %v", err)
	}
	if activeAfter != 2 {
		t.Fatalf("expected 2 active sessions after cleanup, got %d", activeAfter)
	}
}

func TestStore_Stats(t *testing.T) {
	t.Parallel()

	s := NewStore()
	now := time.Now()

	earliest := now.Add(-2 * time.Minute)
	latest := now.Add(3 * time.Minute)

	_ = s.Upsert(context.Background(), &domain.Session{ID: "expired", State: "x", ExpiresAt: earliest})
	_ = s.Upsert(context.Background(), &domain.Session{ID: "active", State: "x", ExpiresAt: latest})
	_ = s.Upsert(context.Background(), &domain.Session{ID: "no-expiry", State: "x"})

	stats, err := s.Stats(context.Background())
	if err != nil {
		t.Fatalf("unexpected stats error: %v", err)
	}

	if stats.Total != 3 || stats.Active != 2 || stats.Expired != 1 || stats.WithoutExpiry != 1 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
	if stats.EarliestExpiry == nil || !stats.EarliestExpiry.Equal(earliest) {
		t.Fatalf("unexpected earliest expiry: %+v", stats.EarliestExpiry)
	}
	if stats.LatestExpiry == nil || !stats.LatestExpiry.Equal(latest) {
		t.Fatalf("unexpected latest expiry: %+v", stats.LatestExpiry)
	}
}
