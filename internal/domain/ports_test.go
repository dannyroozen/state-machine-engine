package domain

import (
	"context"
	"testing"
)

type testConfigProvider struct{}

func (testConfigProvider) LoadStateMachine(context.Context) (*StateMachine, error) {
	return &StateMachine{}, nil
}
func (testConfigProvider) LoadRuntimeConfig(context.Context) (*RuntimeConfig, error) {
	return &RuntimeConfig{}, nil
}

type testSessionStore struct{}

func (testSessionStore) Get(context.Context, string) (*Session, error) { return &Session{}, nil }
func (testSessionStore) Upsert(context.Context, *Session) error        { return nil }

type testConditionEvaluator struct{}

func (testConditionEvaluator) Evaluate(context.Context, string, RequestEnvelope, *Session) (bool, error) {
	return true, nil
}

type testActionExecutor struct{}

func (testActionExecutor) Execute(context.Context, string, RequestEnvelope, *Session, TransitionEvent) ([]byte, error) {
	return nil, nil
}

type testObserver struct{}

func (testObserver) OnTransition(context.Context, TransitionEvent) error { return nil }

type testValidator struct{}

func (testValidator) Validate(*StateMachine, EngineDependencies) error { return nil }

func TestPorts_CompileTimeImplementations(t *testing.T) {
	t.Parallel()

	var _ ConfigProvider = testConfigProvider{}
	var _ SessionStore = testSessionStore{}
	var _ ConditionEvaluator = testConditionEvaluator{}
	var _ ActionExecutor = testActionExecutor{}
	var _ Observer = testObserver{}
	var _ Validator = testValidator{}
}
