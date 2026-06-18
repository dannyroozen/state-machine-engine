package engine

import (
	"context"
	"errors"
	"fmt"
	"state-machine-engine/internal/logging"
	"strings"

	"state-machine-engine/internal/domain"
)

var logger = logging.NewLogger("engine")

type Engine struct {
	// conditions is the part of the engine that simply evaluates transition conditions for true/false
	conditions domain.ConditionEvaluator
	// actions is the part of the engine that will execute transition actions before moving states
	actions domain.ActionExecutor
	// observers execute at each transition to observe what's happening, for example for logging and metrics
	observers []domain.Observer
}

func NewEngine(conditions domain.ConditionEvaluator, actions domain.ActionExecutor, observers []domain.Observer) *Engine {
	return &Engine{
		conditions: conditions,
		actions:    actions,
		observers:  observers,
	}
}

func (e *Engine) HasConditionEvaluator() bool {
	return e != nil && e.conditions != nil
}

func (e *Engine) HasActionExecutor() bool {
	return e != nil && e.actions != nil
}

func (e *Engine) ObserverCount() int {
	if e == nil {
		return 0
	}
	return len(e.observers)
}

func (e *Engine) HasObserver(index int) bool {
	if e == nil || index < 0 || index >= len(e.observers) {
		return false
	}
	return e.observers[index] != nil
}

func (e *Engine) Step(
	ctx context.Context,
	machine *domain.StateMachine,
	req domain.RequestEnvelope,
	session *domain.Session,
) (output []byte, err error) {

	if machine == nil {
		return nil, errors.New("required dependency is not configured: machine is nil")
	} else if session == nil {
		return nil, errors.New("required dependency is not configured: session is nil")
	}

	fromState := session.State
	if strings.ToLower(fromState) == domain.ExitState {
		return nil, fmt.Errorf("cannot continue from %s state", domain.ExitState)
	}

	// Upfront validator guarantees state exists, no need to double-check
	stateDef := machine.States[fromState]

	// Find the next state we should transition to
	var selected *domain.TransitionDefinition
	for i := range stateDef.Transitions {
		// so we can correctly assign a pointer to selected when found
		transition := &stateDef.Transitions[i]

		// e.conditions assumed to exist be validated on engine startup
		match, evalErr := e.conditions.Evaluate(ctx, transition.Condition, req, session)
		if evalErr != nil {
			// error caught when evaluating transition condition, proceed to error state
			session.State = machine.ErrorState
			_ = e.notify(ctx, domain.TransitionEvent{
				SessionID:   session.ID,
				MachineName: machine.Name,
				FromState:   fromState,
				Transition:  transition.ID,
				ToState:     machine.ErrorState,
				Action:      transition.Action,
				Success:     false,
				Error:       evalErr.Error(),
			})
			return nil, evalErr
		}
		if match {
			selected = transition
			break
		}
	}

	if selected == nil {
		// TODO: This should trigger a transition to the error state
		return nil, errors.New("no transition matched")
	}

	// Execute the action associated with the transition, if any
	var out []byte
	if selected.Action != "" {
		// e.actions assumed to exist and be validated on engine startup
		out, err = e.actions.Execute(ctx, selected.Action, req, session)
		if err != nil {
			session.State = machine.ErrorState
			_ = e.notify(ctx, domain.TransitionEvent{
				SessionID:   session.ID,
				MachineName: machine.Name,
				FromState:   fromState,
				Transition:  selected.ID,
				ToState:     machine.ErrorState,
				Action:      selected.Action,
				Success:     false,
				Error:       err.Error(),
			})
			return nil, fmt.Errorf("action failed: %v", err)
		}
	}

	output = out
	targetState := selected.Target
	session.State = selected.Target
	// Observe the transition
	_ = e.notify(ctx, domain.TransitionEvent{
		SessionID:   session.ID,
		MachineName: machine.Name,
		FromState:   fromState,
		Transition:  selected.ID,
		ToState:     selected.Target,
		Action:      selected.Action,
		Success:     true,
	})

	// Unless we've hit the exit state, we should execute any associated action connected with the target state.
	if strings.ToLower(targetState) != domain.ExitState {
		targetStateDef := machine.States[targetState]
		if targetStateDef.Action != "" {
			out, err = e.actions.Execute(ctx, targetStateDef.Action, req, session)
			if err != nil {
				session.State = machine.ErrorState
				_ = e.notify(ctx, domain.TransitionEvent{
					SessionID:   session.ID,
					MachineName: machine.Name,
					FromState:   targetState,
					Transition:  "runtime_invalid_state",
					ToState:     machine.ErrorState,
					Success:     false,
					Error:       "target state missing from machine definition",
				})
				return nil, fmt.Errorf("action failed: state %q not found", fromState)
			}
			// combine output with whatever output we may have received from the action on the transition leading to this state.
			// it is up to the state machine definition to make sure either actions on transitions don't output,
			// or that the output works with the output of the state
			output = append(output, out...)
		}
	}

	return
}

func (e *Engine) notify(ctx context.Context, event domain.TransitionEvent) error {
	for _, obs := range e.observers {
		if obs == nil {
			continue
		}
		if err := obs.OnTransition(ctx, event); err != nil {
			logger.Error(fmt.Sprintf(
				"observer notify failed (session=%s machine=%s from=%s transition=%s to=%s action=%s): %v",
				event.SessionID,
				event.MachineName,
				event.FromState,
				event.Transition,
				event.ToState,
				event.Action,
				err,
			))
			// TODO: In a future version, perhaps attempt to run all observers, maybe asynchronously, and collect the errors at the end
			return err
		}
	}
	return nil
}
