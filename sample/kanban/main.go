package main

import (
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
	conditions.Register("input_exists", conditionsregistry.InputExistsCondition)
	conditions.Register("input_equals", conditionsregistry.InputEqualsCondition)

	actions := registry.NewExecutor()
	actions.Register("noop", registry.NoopAction)
	actions.Register("echo_input", registry.EchoInputAction)
	actions.Register("force_error", registry.ForceErrorAction)

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

func main() {
	rt := NewKanbanRuntime()
	cmd.Execute(rt)
}
