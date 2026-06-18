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
