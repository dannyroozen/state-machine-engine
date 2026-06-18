package engine

import (
	"context"
	"errors"
	"strings"
	"testing"

	"state-machine-engine/internal/domain"
)

type condStub struct {
	match bool
	err   error
}

func (c condStub) Evaluate(context.Context, string, domain.RequestEnvelope, *domain.Session) (bool, error) {
	return c.match, c.err
}

type actionStub struct {
	out []byte
	err error
}

func (a actionStub) Execute(context.Context, string, domain.RequestEnvelope, *domain.Session, domain.TransitionEvent) ([]byte, error) {
	return a.out, a.err
}

type obsStub struct {
	called int
}

func (o *obsStub) OnTransition(context.Context, domain.TransitionEvent) error {
	o.called++
	return nil
}

func baseMachine() *domain.StateMachine {
	return &domain.StateMachine{
		Name:       "m1",
		Initial:    "start",
		ErrorState: "error",
		States: map[string]domain.StateDefinition{
			"start": {Transitions: []domain.TransitionDefinition{{ID: "t1", Condition: "always", Target: "next"}}},
			"next":  {Transitions: []domain.TransitionDefinition{{ID: "t2", Condition: "always", Target: "exit"}}},
			"error": {Transitions: []domain.TransitionDefinition{{ID: "te", Target: "exit"}}},
			"exit":  {},
		},
	}
}

func TestStep_Success(t *testing.T) {
	t.Parallel()

	obs := &obsStub{}
	e := NewEngine(condStub{match: true}, actionStub{}, []domain.Observer{obs})
	s := &domain.Session{ID: "s1", State: "start"}

	out, err := e.Step(context.Background(), baseMachine(), domain.RequestEnvelope{SessionID: "s1"}, s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != nil {
		t.Fatalf("expected nil output, got %s", string(out))
	}
	if s.State != "next" {
		t.Fatalf("expected state next, got %s", s.State)
	}
	if obs.called != 1 {
		t.Fatalf("expected observer called once, got %d", obs.called)
	}
}

func TestStep_NoTransitionMatched(t *testing.T) {
	t.Parallel()

	e := NewEngine(condStub{match: false}, actionStub{}, nil)
	s := &domain.Session{ID: "s1", State: "start"}

	_, err := e.Step(context.Background(), baseMachine(), domain.RequestEnvelope{SessionID: "s1"}, s)
	if err == nil || !strings.Contains(err.Error(), "no transition matched") {
		t.Fatalf("expected ErrNoTransitionMatched, got %v", err)
	}
}

func TestStep_ConditionErrorMovesToErrorState(t *testing.T) {
	t.Parallel()

	e := NewEngine(condStub{err: errors.New("boom")}, actionStub{}, nil)
	s := &domain.Session{ID: "s1", State: "start"}

	_, err := e.Step(context.Background(), baseMachine(), domain.RequestEnvelope{SessionID: "s1"}, s)
	if err == nil {
		t.Fatal("expected error")
	}
	if s.State != "error" {
		t.Fatalf("expected state error, got %s", s.State)
	}
}

func TestStep_ActionErrorMovesToErrorState(t *testing.T) {
	t.Parallel()

	m := baseMachine()
	m.States["start"] = domain.StateDefinition{
		Transitions: []domain.TransitionDefinition{
			{ID: "t1", Condition: "always", Target: "next", Action: "a1"},
		},
	}

	e := NewEngine(condStub{match: true}, actionStub{err: errors.New("action failed")}, nil)
	s := &domain.Session{ID: "s1", State: "start"}

	_, err := e.Step(context.Background(), m, domain.RequestEnvelope{SessionID: "s1"}, s)
	if err == nil || !strings.Contains(err.Error(), "action failed") {
		t.Fatalf("expected ErrActionFailed, got %v", err)
	}
	if s.State != "error" {
		t.Fatalf("expected state error, got %s", s.State)
	}
}
