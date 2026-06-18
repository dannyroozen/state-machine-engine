package servers

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"state-machine-engine/internal/app"
	"state-machine-engine/internal/domain"
)

type cfgStub struct{}

func (cfgStub) LoadStateMachine(context.Context) (*domain.StateMachine, error) {
	return &domain.StateMachine{
		Name:       "m1",
		Initial:    "start",
		ErrorState: "error",
		States: map[string]domain.StateDefinition{
			"start": {Transitions: []domain.TransitionDefinition{{ID: "t1", Condition: "always", Target: "exit"}}},
			"error": {Transitions: []domain.TransitionDefinition{{ID: "te", Target: "exit"}}},
			"exit":  {},
		},
	}, nil
}
func (cfgStub) LoadRuntimeConfig(context.Context) (*domain.RuntimeConfig, error) {
	return &domain.RuntimeConfig{SessionTTL: time.Minute}, nil
}

type storeStub struct {
	s *domain.Session
}

func (s *storeStub) Get(context.Context, string) (*domain.Session, error) { return s.s, nil }
func (s *storeStub) Upsert(context.Context, *domain.Session) error        { return nil }

type validatorStub struct{}

func (validatorStub) Validate(*domain.StateMachine, domain.EngineDependencies) error { return nil }

type condStub struct{}

func (condStub) Evaluate(context.Context, string, domain.RequestEnvelope, *domain.Session) (bool, error) {
	return true, nil
}

type actionStub struct{}

func (actionStub) Execute(context.Context, string, domain.RequestEnvelope, *domain.Session) ([]byte, error) {
	return nil, nil
}

func testService() *app.Service {
	return app.NewService(cfgStub{}, &storeStub{}, validatorStub{}, condStub{}, actionStub{}, nil)
}

func TestHandleRequest_Success(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/requests", http.NoBody)
	req.Body = http.NoBody
	rr := httptest.NewRecorder()

	handleRequest(testService(), rr, req)

	// empty body -> invalid_json
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestHandleRequest_InvalidJSON(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/requests", strings.NewReader("{\"session_id\":"))
	rr := httptest.NewRecorder()

	handleRequest(testService(), rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestWriteError(t *testing.T) {
	t.Parallel()

	rr := httptest.NewRecorder()
	writeError(rr, http.StatusTeapot, "x", "y")

	if rr.Code != http.StatusTeapot {
		t.Fatalf("status mismatch: %d", rr.Code)
	}
	var resp domain.ResponseEnvelope
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if resp.Error == nil || resp.Error.Code != "x" || resp.Error.Message != "y" {
		t.Fatalf("unexpected error payload: %+v", resp.Error)
	}
}

func TestStartAndListen_AlreadyRunning(t *testing.T) {
	t.Parallel()

	socketPath := filepath.Join(os.TempDir(), "state-machine-engine.sock")

	if conn, err := net.DialTimeout("unix", socketPath, time.Second); err == nil {
		_ = conn.Close()
		t.Fatalf("server already running on %s", socketPath)
	}
	if err := os.Remove(socketPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed to remove stale socket: %v", err)
	}

	// Start server listener
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to open test socket: %v", err)
	}
	defer func() {
		_ = ln.Close()
	}()

	err = StartAndListen(socketPath, testService())
	if err == nil {
		t.Fatal("expected already running error")
	}
	if !errors.Is(err, err) { // keep vet happy while still checking message below
		t.Fatal("unexpected nil-ish error")
	}
}
