package validation

import (
	"errors"
	"fmt"

	"state-machine-engine/internal/domain"
)

var (
	ErrDependencyMissing   = errors.New("required dependency is not configured")
	ErrStateMachineInvalid = errors.New("state machine definition invalid")
)

type StateMachineValidator struct{}

func NewStateMachineValidator() *StateMachineValidator {
	return &StateMachineValidator{}
}

func (v *StateMachineValidator) Validate(machine *domain.StateMachine, engineDeps domain.EngineDependencies) error {
	var errs []error

	if engineDeps == nil {
		errs = append(errs, fmt.Errorf("%w: engine dependencies missing", ErrDependencyMissing))
	} else {
		if !engineDeps.HasConditionEvaluator() {
			errs = append(errs, fmt.Errorf("%w: missing condition evaluator", ErrDependencyMissing))
		}
		if !engineDeps.HasActionExecutor() {
			errs = append(errs, fmt.Errorf("%w: missing action executor", ErrDependencyMissing))
		}
		for i := 0; i < engineDeps.ObserverCount(); i++ {
			if !engineDeps.HasObserver(i) {
				errs = append(errs, fmt.Errorf("%w: observer[%d] is nil", ErrDependencyMissing, i))
			}
		}
	}

	// We'll evaluate the machine, too, even if the engine is not right, so we can report on everything at once.
	if machine == nil {
		errs = append(errs, fmt.Errorf("%w: nil machine", ErrStateMachineInvalid))
		return errors.Join(errs...) // cannot continue
	}
	if machine.Name == "" {
		errs = append(errs, fmt.Errorf("%w: machine name is required", ErrStateMachineInvalid))
	}
	if machine.Initial == "" {
		errs = append(errs, fmt.Errorf("%w: initial_state is required", ErrStateMachineInvalid))
	}
	if machine.ErrorState == "" {
		errs = append(errs, fmt.Errorf("%w: error_state is required", ErrStateMachineInvalid))
	}
	if len(machine.States) == 0 {
		errs = append(errs, fmt.Errorf("%w: at least one state is required", ErrStateMachineInvalid))
		return errors.Join(errs...)
	}

	// Initial state and error state should exist on the state machine
	if _, ok := machine.States[machine.Initial]; !ok {
		errs = append(errs, fmt.Errorf("%w: initial_state %q not found", ErrStateMachineInvalid, machine.Initial))
	}
	if _, ok := machine.States[machine.ErrorState]; !ok {
		errs = append(errs, fmt.Errorf("%w: error_state %q not found", ErrStateMachineInvalid, machine.ErrorState))
	}

	// Validate each state + transitions
	for stateName, state := range machine.States {
		if len(state.Transitions) == 0 && stateName != "exit" { // allow exit state without outgoing transitions
			errs = append(errs, fmt.Errorf("%w: state %q has no transitions", ErrStateMachineInvalid, stateName))
			continue
		}

		for i, tr := range state.Transitions {
			if tr.ID == "" {
				errs = append(errs, fmt.Errorf("%w: state %q transition[%d] missing id", ErrStateMachineInvalid, stateName, i))
			}
			if tr.Target == "" {
				errs = append(errs, fmt.Errorf("%w: state %q transition %q missing target", ErrStateMachineInvalid, stateName, tr.ID))
			} else if _, ok := machine.States[tr.Target]; !ok {
				errs = append(errs, fmt.Errorf("%w: state %q transition %q target %q not found",
					ErrStateMachineInvalid, stateName, tr.ID, tr.Target))
			}
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}
