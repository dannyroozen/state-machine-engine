package domain

import "context"

// ConfigProvider implemented by a configuration plugin.
// For example, implement configuration provided in a relational database instead of a config file
// Provides state machine definition and runtime configuration
type ConfigProvider interface {
	LoadStateMachine(ctx context.Context) (*StateMachine, error)
	LoadRuntimeConfig(ctx context.Context) (*RuntimeConfig, error)
}

// SessionStore implemented by a session plugin.
// For example, session may be stored in elasticache, dynamodb, files on the system, or simply in memory.
// Provides getter and upsert functions.
type SessionStore interface {
	Get(ctx context.Context, sessionID string) (*Session, error)
	Upsert(ctx context.Context, session *Session) error
}

// ConditionEvaluator implemented by a plugin,
// in case downstream implementations want to provide extra tooling or behavior when evaluating conditions.
type ConditionEvaluator interface {
	Evaluate(ctx context.Context, condition string, req RequestEnvelope, session *Session) (bool, error)
}

// ActionExecutor implemented by a plugin,
// in case downstream implementations want to provide extra tooling or behavior when executing an action.
type ActionExecutor interface {
	Execute(ctx context.Context, actionName string, req RequestEnvelope, session *Session, event TransitionEvent) (jsonOutput []byte, err error)
}

// Observer to provide behavior that happens when transitioning between states
type Observer interface {
	OnTransition(ctx context.Context, event TransitionEvent) error
}

// Validator will ensure a valid state machine definition is provided. May be overridden.
type Validator interface {
	Validate(machine *StateMachine, engineDeps EngineDependencies) error
}

type EngineDependencies interface {
	HasConditionEvaluator() bool
	HasActionExecutor() bool
	HasObserver(index int) bool
	ObserverCount() int
}

type TransitionEvent struct {
	SessionID   string `json:"session_id"`
	MachineName string `json:"machine_name"`
	FromState   string `json:"from_state"`
	Transition  string `json:"transition"`
	ToState     string `json:"to_state"`
	Action      string `json:"action,omitempty"`
	Success     bool   `json:"success"`
	Error       string `json:"error,omitempty"`
}
