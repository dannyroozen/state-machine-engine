package main

import (
	"context"
	"fmt"
	"os"
	"state-machine-engine/adapters/actions/registry"
	conditionsregistry "state-machine-engine/adapters/conditions/registry"
	fileconfig "state-machine-engine/adapters/config/json"
	logobserver "state-machine-engine/adapters/observers/logging"
	"state-machine-engine/adapters/session/memory"
	"state-machine-engine/cmd"
	"state-machine-engine/internal/app"
	"state-machine-engine/internal/bootstrap"
	"state-machine-engine/internal/domain"
	"state-machine-engine/internal/engine"
	"state-machine-engine/internal/logging"
	"state-machine-engine/internal/validation"
)

var logger = logging.NewLogger("kanban")

type KanbanRuntime struct {
	*bootstrap.DefaultRuntime
}

func NewKanbanRuntime() *KanbanRuntime {
	return &KanbanRuntime{
		DefaultRuntime: bootstrap.NewDefaultRuntime(),
	}
}

func (r *KanbanRuntime) BuildService(machinePath, runtimePath string) *app.Service {
	// TODO: Supply conditions and actions specific to the Kanban Board state machine.
	configProvider := fileconfig.NewProvider(machinePath, runtimePath)
	sessionStore := memory.NewStore()
	validator := validation.NewStateMachineValidator()

	conditions := conditionsregistry.NewEvaluator()
	conditions.Register("always", conditionsregistry.AlwaysCondition)
	conditions.Register("input_equals", conditionsregistry.InputEqualsCondition)

	actions := registry.NewExecutor()
	actions.Register("noop", registry.NoopAction)
	actions.Register("echo_input", registry.EchoInputAction)
	actions.Register("echo_target", EchoTargetAction)
	actions.Register("echo_state_name", EchoStateNameAction)

	observers := []domain.Observer{
		logobserver.NewObserver(os.Stdout),
	}

	machine, err := configProvider.LoadStateMachine(nil)
	if err != nil {
		logger.Fatal(fmt.Sprintf("failed to load state machine: %v", err))
	}
	if err = validator.Validate(machine, engine.NewEngine(conditions, actions, observers)); err != nil {
		logger.Fatal(fmt.Sprintf("failed to validate state machine: %v", err))
	}

	return app.NewService(configProvider, sessionStore, validator, conditions, actions, observers)
	// return r.DefaultRuntime.BuildService(machinePath, runtimePath) // optional fallback
}

func EchoTargetAction(_ context.Context, _ domain.RequestEnvelope, sess *domain.Session, event domain.TransitionEvent) ([]byte, error) {
	logger.Debug("To State: " + event.ToState)

	// JSON-shaped payload emitted as plain text.
	transitionOut := fmt.Sprintf(`{"from":"%s","to":"%s"}`, event.FromState, event.ToState)

	// Save transition output for the state-level action to consume.
	sess.Data = []byte(transitionOut) // session data can/should be all of the data needed to maintain this session, but this is just for illustrative purposes

	// Important: return nil so engine does not concatenate two JSON documents.
	return nil, nil
}

func EchoStateNameAction(_ context.Context, _ domain.RequestEnvelope, sess *domain.Session, _ domain.TransitionEvent) ([]byte, error) {
	previous := "null"
	if len(sess.Data) > 0 {
		previous = string(sess.Data)
	}

	// JSON-shaped payload emitted as plain text.
	out := fmt.Sprintf(`{"action_output":%s,"current_state":"%s"}`, previous, sess.State)
	return []byte(out), nil
}

func main() {
	rt := NewKanbanRuntime()
	cmd.Execute(rt)
}
