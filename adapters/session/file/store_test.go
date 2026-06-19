package file

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"state-machine-engine/internal/domain"
)

func TestStore_GetMissing(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	s, err := NewStore(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := s.Get(context.Background(), "missing")
	if err != nil {
		t.Fatalf("unexpected get error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
}

func TestStore_UpsertAndGet_CopySemantics(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	s, err := NewStore(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	orig := &domain.Session{
		ID:        "s1",
		State:     "start",
		Data:      json.RawMessage(`{"a":1}`),
		ExpiresAt: time.Now().Add(time.Minute),
	}
	if err = s.Upsert(context.Background(), orig); err != nil {
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
	orig.Data[0] = '['

	got2, err := s.Get(context.Background(), "s1")
	if err != nil {
		t.Fatalf("unexpected second get error: %v", err)
	}
	if got2.State != "start" {
		t.Fatalf("expected persisted state to remain 'start', got %q", got2.State)
	}
}

func TestStore_UpsertNil(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	s, err := NewStore(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = s.Upsert(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "invalid input: nil session") {
		t.Fatalf("expected nil session error, got %v", err)
	}
}

func TestStore_GetCorruptJSON(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	s, err := NewStore(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	path := s.sessionPath("bad-json")
	if err = os.WriteFile(path, []byte(`{"id":"x",`), 0o600); err != nil {
		t.Fatalf("failed writing corrupt session: %v", err)
	}

	_, err = s.Get(context.Background(), "bad-json")
	if err == nil || !strings.Contains(err.Error(), "invalid session json") {
		t.Fatalf("expected corrupt json error, got %v", err)
	}
}

func TestStore_RespectsContextWhileWaitingForLock(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	s, err := NewStore(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	base := s.sessionPath("s1")
	lockPath := base + ".lock"
	if err = os.WriteFile(lockPath, []byte("locked"), 0o600); err != nil {
		t.Fatalf("failed creating lock file: %v", err)
	}
	defer os.Remove(lockPath)

	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()

	err = s.Upsert(ctx, &domain.Session{ID: "s1", State: "x"})
	if err == nil {
		t.Fatalf("expected context timeout/cancel error")
	}
}

func TestStore_HashedFilename(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	s, err := NewStore(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err = s.Upsert(context.Background(), &domain.Session{
		ID:    "my-session-id",
		State: "created",
	}); err != nil {
		t.Fatalf("unexpected upsert error: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("unexpected readdir error: %v", err)
	}

	foundJSON := false
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".json") {
			foundJSON = true
			if strings.Contains(e.Name(), "my-session-id") {
				t.Fatalf("session filename should not contain raw session id: %s", e.Name())
			}
			if filepath.Ext(e.Name()) != ".json" {
				t.Fatalf("expected .json extension, got %s", e.Name())
			}
		}
	}
	if !foundJSON {
		t.Fatalf("expected persisted session json file")
	}
}
