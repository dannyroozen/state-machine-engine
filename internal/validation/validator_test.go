package validation

import (
	"errors"
	"testing"

	"state-machine-engine/internal/domain"
)

type engineDepsStub struct {
	hasConditions bool
	hasActions    bool
	observers     []bool
}

func (e engineDepsStub) HasConditionEvaluator() bool { return e.hasConditions }
func (e engineDepsStub) HasActionExecutor() bool     { return e.hasActions }
func (e engineDepsStub) ObserverCount() int          { return len(e.observers) }
func (e engineDepsStub) HasObserver(index int) bool {
	if index < 0 || index >= len(e.observers) {
		return false
	}
	return e.observers[index]
}

func TestValidate_ValidMachine(t *testing.T) {
	t.Parallel()

	v := NewStateMachineValidator()
	m := &domain.StateMachine{
		Name:       "m1",
		Initial:    "start",
		ErrorState: "error",
		States: map[string]domain.StateDefinition{
			"start": {
				Transitions: []domain.TransitionDefinition{
					{ID: "t1", Condition: "always", Target: "exit"},
				},
			},
			"error": {Transitions: []domain.TransitionDefinition{{ID: "te", Target: "exit"}}},
			"exit":  {Transitions: nil},
		},
	}

	deps := engineDepsStub{hasConditions: true, hasActions: true, observers: []bool{true}}
	if err := v.Validate(m, deps); err != nil {
		t.Fatalf("expected valid machine, got %v", err)
	}
}

func TestValidate_ValidMachine_NoObservers(t *testing.T) {
	t.Parallel()

	v := NewStateMachineValidator()
	m := &domain.StateMachine{
		Name:       "m1",
		Initial:    "start",
		ErrorState: "error",
		States: map[string]domain.StateDefinition{
			"start": {
				Transitions: []domain.TransitionDefinition{
					{ID: "t1", Condition: "always", Target: "exit"},
				},
			},
			"error": {Transitions: []domain.TransitionDefinition{{ID: "te", Target: "exit"}}},
			"exit":  {Transitions: nil},
		},
	}

	deps := engineDepsStub{hasConditions: true, hasActions: true}
	if err := v.Validate(m, deps); err != nil {
		t.Fatalf("expected valid machine, got %v", err)
	}
}

func TestValidate_NilMachine(t *testing.T) {
	t.Parallel()

	v := NewStateMachineValidator()
	deps := engineDepsStub{hasConditions: true, hasActions: true}
	err := v.Validate(nil, deps)
	if err == nil || !errors.Is(err, ErrStateMachineInvalid) {
		t.Fatalf("expected ErrStateMachineInvalid, got %v", err)
	}
}

func TestValidate_InvalidMachine(t *testing.T) {
	t.Parallel()

	v := NewStateMachineValidator()
	m := &domain.StateMachine{
		Name:       "",
		Initial:    "",
		ErrorState: "",
		States: map[string]domain.StateDefinition{
			"start": {Transitions: []domain.TransitionDefinition{{ID: "", Target: ""}}},
		},
	}

	deps := engineDepsStub{hasConditions: true, hasActions: true}
	err := v.Validate(m, deps)
	if err == nil || !errors.Is(err, ErrStateMachineInvalid) {
		t.Fatalf("expected ErrStateMachineInvalid, got %v", err)
	}
}
