package engine

import (
	"context"
	"errors"
	"fmt"
	"state-machine-engine/internal/logging"
	"strings"

	"state-machine-engine/internal/domain"
)

// TODO: Every application should have reporting, but is that up to the implementing application to include in their state machine? Can we provide some defaults?

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
	logger.Debug(fmt.Sprintf("step: from state: %s", fromState))
	if strings.ToLower(fromState) == domain.ExitState {
		return nil, fmt.Errorf("cannot continue from %s state", domain.ExitState)
	}

	// Upfront validator guarantees state exists, no need to double-check
	stateDef := machine.States[fromState]

	// ## Find the next state we should transition to ##
	var selected *domain.TransitionDefinition
	var success = true

	for i := range stateDef.Transitions {
		// so we can correctly assign a pointer to selected when found
		transition := &stateDef.Transitions[i]

		// e.conditions assumed to exist be validated on engine startup
		var match bool
		match, err = e.conditions.Evaluate(ctx, transition.Condition, req, session)
		if err != nil {
			logger.Error(fmt.Sprintf("error caught when evaluating transition condition: %v", err))
			// error caught when evaluating transition condition, proceed to error state
			session.State = machine.ErrorState

			selected = &domain.TransitionDefinition{
				ID:     transition.ID,
				Target: machine.ErrorState,
			}

			success = false
			break // we don't want to return yet; we need to execute the error state action, if any
		}
		if match {
			logger.Debug(fmt.Sprintf("step: selected transition[index %d]: %s", i, transition.ID))
			selected = transition
			break
		}
	}

	// ## Check for validity ##
	if err == nil && selected == nil { // no transition matched, but no error caught
		logger.Warn("no transition matched, no default transition found")

		// add error state transition to avoid nil checks on selected later
		selected = &domain.TransitionDefinition{
			ID:     "engine_errorNoTransitions",
			Target: machine.ErrorState,
		}

		session.State = machine.ErrorState
		success = false
	}

	// ## Execute the Action associated with the transition, if any
	var out []byte
	if err == nil && selected.Action != "" {
		logger.Debug(fmt.Sprintf("Executing action: [%s] ", selected.Action))
		// e.actions assumed to exist and be validated on engine startup
		out, err = e.actions.Execute(ctx, selected.Action, req, session,
			// give the action some context, so it knows things like which state we're going to
			domain.TransitionEvent{
				SessionID:   session.ID,
				MachineName: machine.Name,
				FromState:   fromState,
				Transition:  selected.ID,
				ToState:     selected.Target,
				Action:      selected.Action,
				Success:     success,
			})

		if err != nil {
			logger.Error(fmt.Sprintf("error caught when evaluating action: %s", err.Error()))
			// error caught when evaluating action, proceed to error state
			session.State = machine.ErrorState

			selected = &domain.TransitionDefinition{
				ID:     "engine_errorNoTransitions",
				Target: machine.ErrorState,
			}

			success = false
		}
	}

	output = out
	targetState := selected.Target
	session.State = selected.Target
	transitionEvent := domain.TransitionEvent{
		SessionID:   session.ID,
		MachineName: machine.Name,
		FromState:   fromState,
		Transition:  selected.ID,
		ToState:     selected.Target,
		Action:      selected.Action,
		Success:     success,
	}
	if err != nil {
		transitionEvent.Error = err.Error()
	}

	// ## Observe the transition ##
	_ = e.notify(ctx, transitionEvent)

	// ## Execute State Action ##
	if strings.ToLower(session.State) != domain.ExitState {
		targetStateDef := machine.States[session.State]
		if targetStateDef.Action != "" {
			out, err = e.actions.Execute(ctx, targetStateDef.Action, req, session, transitionEvent)
			if err != nil {
				session.State = machine.ErrorState
				_ = e.notify(ctx, domain.TransitionEvent{
					SessionID:   session.ID,
					MachineName: machine.Name,
					FromState:   targetState,
					Transition:  "runtime_error_executing_state",
					ToState:     machine.ErrorState,
					Success:     false,
					Error:       err.Error(),
				})
				return nil, fmt.Errorf("action failed: when executing state [%q]: %v", fromState, err)
			}
			// combine output with whatever output we may have received from the action on the transition leading to this state.
			// it is up to the state machine definition to make sure either actions on transitions don't output,
			// or that the output works with the output of the state
			output = append(output, out...)
		}
	}

	return
}

// notify all observers
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
