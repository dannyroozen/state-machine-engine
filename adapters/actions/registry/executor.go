package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"state-machine-engine/internal/domain"
)

type ActionFunc func(ctx context.Context, req domain.RequestEnvelope, session *domain.Session, event domain.TransitionEvent) ([]byte, error)

type Executor struct {
	mu      sync.RWMutex
	actions map[string]ActionFunc
}

func NewExecutor() *Executor {
	return &Executor{
		actions: make(map[string]ActionFunc),
	}
}

// Register an action for use with the application's state machine
// Thread-safe
func (e *Executor) Register(name string, fn ActionFunc) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.actions[name] = fn
}

// Execute executes the action with the given name and returns the output or an error
// Thread-safe
func (e *Executor) Execute(ctx context.Context, actionName string, req domain.RequestEnvelope, session *domain.Session, event domain.TransitionEvent) ([]byte, error) {
	e.mu.RLock()
	fn, ok := e.actions[actionName]
	e.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown action %q", actionName)
	}
	return fn(ctx, req, session, event)
}

// Example actions provided for this plugin. Implementations could use this same actions registry/executor but provide their own action functions.

// NoopAction is an example function that literally does nothing. Why would you do this?
func NoopAction(_ context.Context, _ domain.RequestEnvelope, _ *domain.Session, _ domain.TransitionEvent) ([]byte, error) {
	return nil, nil
}

// EchoInputAction simply puts the input from the request into the output as json
func EchoInputAction(_ context.Context, req domain.RequestEnvelope, _ *domain.Session, _ domain.TransitionEvent) ([]byte, error) {
	out := map[string]json.RawMessage{
		"echo": req.Input,
	}
	return json.Marshal(out)
}

// ForceErrorAction is an example function that will force an error to be emitted and transition us to the error state
func ForceErrorAction(_ context.Context, _ domain.RequestEnvelope, _ *domain.Session, _ domain.TransitionEvent) ([]byte, error) {
	return nil, fmt.Errorf("forced action error")
}

// EchoInputTextAction emits plain text (non-JSON) output to demonstrate flexible transport encoding.
// It is up to the server (e.g. unix.go) to handle validation of output and proper transport encoding.
func EchoInputTextAction(_ context.Context, req domain.RequestEnvelope, _ *domain.Session, _ domain.TransitionEvent) ([]byte, error) {
	return []byte(fmt.Sprintf("echo: %s", string(req.Input))), nil
}

// EmitBinaryAction emits arbitrary binary bytes to demonstrate base64 transport encoding.
// It is up to the server (e.g. unix.go) to handle validation of output and proper transport encoding.
func EmitBinaryAction(_ context.Context, _ domain.RequestEnvelope, _ *domain.Session, _ domain.TransitionEvent) ([]byte, error) {
	return []byte{0xDE, 0xAD, 0xBE, 0xEF}, nil
}
