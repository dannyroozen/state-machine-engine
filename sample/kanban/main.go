package main

import (
	"context"
	"fmt"
	"os"
	"state-machine-engine/adapters/actions/registry"
	conditionsregistry "state-machine-engine/adapters/conditions/registry"
	fileconfig "state-machine-engine/adapters/config/json"
	logobserver "state-machine-engine/adapters/observers/logging"
	filesession "state-machine-engine/adapters/session/file"
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
	configProvider := fileconfig.NewProvider(machinePath, runtimePath)

	var sessionStore domain.SessionStore = memory.NewStore()

	if rtCfg, err := configProvider.LoadRuntimeConfig(context.Background()); err == nil && rtCfg != nil && rtCfg.Session.FileLocation != "" {
		if fsStore, fsErr := filesession.NewStore(rtCfg.Session.FileLocation); fsErr == nil {
			sessionStore = fsStore
		} else {
			logger.Error(fmt.Sprintf("failed to instantiate file session store: %v", fsErr))
			// TODO: Should this be fatal? Or go ahead and use memory store as backup?
		}
	} else if err != nil {
		logger.Error(fmt.Sprintf("failed to load runtime config: %v", err))
	}

	validator := validation.NewStateMachineValidator()

	conditions := conditionsregistry.NewEvaluator()
	conditions.Register("always", conditionsregistry.AlwaysCondition)
	conditions.Register("input_equals", conditionsregistry.InputEqualsCondition)

	actions := registry.NewExecutor()
	actions.Register("noop", registry.NoopAction)
	actions.Register("echo_input", registry.EchoInputAction)
	// Add a couple of actions specific to the Kanban Board state machine.
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

	return app.NewService(configProvider, configProvider, sessionStore, validator, conditions, actions, observers)
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
