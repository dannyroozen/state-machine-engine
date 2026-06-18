package bootstrap

import (
	"context"
	"fmt"
	"os"
	"state-machine-engine/adapters/actions/registry"
	conditionsregistry "state-machine-engine/adapters/conditions/registry"
	fileconfig "state-machine-engine/adapters/config/json"
	logobserver "state-machine-engine/adapters/observers/logging"
	"state-machine-engine/adapters/servers"
	"state-machine-engine/adapters/session/memory"
	"state-machine-engine/internal/app"
	bootstrapconfig "state-machine-engine/internal/bootstrap/config"
	"state-machine-engine/internal/domain"
	"state-machine-engine/internal/engine"
	"state-machine-engine/internal/logging"
	"state-machine-engine/internal/validation"
)

var logger = logging.NewLogger("bootstrap")

type DefaultRuntime struct{}

func NewDefaultRuntime() *DefaultRuntime {
	return &DefaultRuntime{}
}

func (r *DefaultRuntime) BuildService(machinePath, runtimePath string) *app.Service {
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
}

func (r *DefaultRuntime) RunRequest(socketPath, input string) error {
	if err := servers.RunClient(socketPath, input); err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	return nil
}

func (r *DefaultRuntime) RunConfigAssistant(machinePath, runtimePath, model string) error {
	ctx := context.Background()

	configProvider := fileconfig.NewProvider(machinePath, runtimePath)
	rtCfg, err := configProvider.LoadRuntimeConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed loading runtime config: %w", err)
	}

	assistant := bootstrapconfig.NewRunAssistant(
		configProvider,
		validation.NewStateMachineValidator(),
		bootstrapconfig.StaticEngineDeps{
			HasConditions: true,
			HasActions:    true,
			Observers:     1,
		},
		os.Stdin,
		os.Stdout,
	)

	if err := assistant.Run(ctx, rtCfg, model); err != nil {
		return fmt.Errorf("config assistant failed: %w", err)
	}
	return nil
}
