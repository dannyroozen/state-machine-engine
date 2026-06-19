package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"state-machine-engine/internal/domain"
)

type cfgStub struct {
	machine *domain.StateMachine
	rt      *domain.RuntimeConfig
	err     error
}

func (c cfgStub) LoadStateMachine(context.Context) (*domain.StateMachine, error) {
	if c.err != nil {
		return nil, c.err
	}
	return c.machine, nil
}
func (c cfgStub) LoadRuntimeConfig(context.Context) (*domain.RuntimeConfig, error) {
	if c.err != nil {
		return nil, c.err
	}
	return c.rt, nil
}

type storeStub struct {
	session *domain.Session
	getErr  error
	putErr  error
}

func (s *storeStub) Get(context.Context, string) (*domain.Session, error) { return s.session, s.getErr }
func (s *storeStub) Upsert(context.Context, *domain.Session) error        { return s.putErr }

type validatorStub struct{ err error }

func (v validatorStub) Validate(*domain.StateMachine, domain.EngineDependencies) error { return v.err }

type condPass struct{}

func (condPass) Evaluate(context.Context, string, domain.RequestEnvelope, *domain.Session) (bool, error) {
	return true, nil
}

type actionNoop struct{}

func (actionNoop) Execute(context.Context, string, domain.RequestEnvelope, *domain.Session, domain.TransitionEvent) ([]byte, error) {
	return nil, nil
}

func testMachine() *domain.StateMachine {
	return &domain.StateMachine{
		Name:       "m1",
		Initial:    "start",
		ErrorState: "error",
		States: map[string]domain.StateDefinition{
			"start": {Transitions: []domain.TransitionDefinition{{ID: "t1", Condition: "always", Target: "exit"}}},
			"error": {Transitions: []domain.TransitionDefinition{{ID: "te", Target: "exit"}}},
			"exit":  {},
		},
	}
}

func TestProcessRequest_Success(t *testing.T) {
	t.Parallel()

	cfg := cfgStub{
		machine: testMachine(),
		rt: &domain.RuntimeConfig{
			Session: domain.SessionRuntimeConfig{TTL: time.Minute},
		},
	}
	store := &storeStub{}
	svc := NewService(cfg, store, validatorStub{}, condPass{}, actionNoop{}, nil)

	resp, err := svc.ProcessRequest(context.Background(), domain.RequestEnvelope{
		SessionID: "s1",
		Input:     []byte(`{"k":"v"}`),
	}, []byte(`{"session_id":"s1","input":{"k":"v"}}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.SessionID != "s1" {
		t.Fatalf("unexpected session id: %s", resp.SessionID)
	}
	if resp.State != "exit" {
		t.Fatalf("expected exit state, got %s", resp.State)
	}
}

func TestProcessRequest_InputValidation(t *testing.T) {
	t.Parallel()

	svc := NewService(
		cfgStub{
			machine: testMachine(),
			rt: &domain.RuntimeConfig{
				Session: domain.SessionRuntimeConfig{TTL: time.Minute},
			},
		},
		&storeStub{},
		validatorStub{},
		condPass{},
		actionNoop{},
		nil,
	)

	_, err := svc.ProcessRequest(context.Background(), domain.RequestEnvelope{SessionID: "", Input: []byte(`{}`)}, []byte(`{}`))
	if !errors.Is(err, ErrMissingSessionID) {
		t.Fatalf("expected ErrMissingSessionID, got %v", err)
	}

	_, err = svc.ProcessRequest(context.Background(), domain.RequestEnvelope{SessionID: "s1", Input: []byte(`{"bad":`)}, []byte(`{}`))
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("expected ErrInvalidRequest, got %v", err)
	}
}

func TestProcessRequest_DependencyMissing(t *testing.T) {
	t.Parallel()

	svc := &Service{}
	_, err := svc.ProcessRequest(context.Background(), domain.RequestEnvelope{SessionID: "s1"}, []byte(`{}`))
	if !errors.Is(err, ErrDependencyMissing) {
		t.Fatalf("expected ErrDependencyMissing, got %v", err)
	}
}

func TestProcessRequest_ExpiredSession(t *testing.T) {
	t.Parallel()

	cfg := cfgStub{
		machine: testMachine(),
		rt: &domain.RuntimeConfig{
			Session: domain.SessionRuntimeConfig{TTL: time.Minute},
		},
	}
	store := &storeStub{
		session: &domain.Session{
			ID:        "s1",
			State:     "start",
			ExpiresAt: time.Now().Add(-time.Minute),
		},
	}
	svc := NewService(cfg, store, validatorStub{}, condPass{}, actionNoop{}, nil)

	_, err := svc.ProcessRequest(context.Background(), domain.RequestEnvelope{SessionID: "s1"}, []byte(`{}`))
	if !errors.Is(err, ErrSessionExpired) {
		t.Fatalf("expected ErrSessionExpired, got %v", err)
	}
}
