package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"state-machine-engine/internal/domain"
)

type ActionFunc func(ctx context.Context, req domain.RequestEnvelope, session *domain.Session) ([]byte, error)

type Executor struct {
	mu      sync.RWMutex
	actions map[string]ActionFunc
}

func NewExecutor() *Executor {
	return &Executor{
		actions: make(map[string]ActionFunc),
	}
}

func (e *Executor) Register(name string, fn ActionFunc) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.actions[name] = fn
}

func (e *Executor) Execute(ctx context.Context, actionName string, req domain.RequestEnvelope, session *domain.Session) ([]byte, error) {
	e.mu.RLock()
	fn, ok := e.actions[actionName]
	e.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown action %q", actionName)
	}
	return fn(ctx, req, session)
}

// Example actions provided for this plugin. Implementations could use this same actions registry/executor but provide their own action functions.

func NoopAction(_ context.Context, _ domain.RequestEnvelope, _ *domain.Session) ([]byte, error) {
	return nil, nil
}

func EchoInputAction(_ context.Context, req domain.RequestEnvelope, _ *domain.Session) ([]byte, error) {
	out := map[string]json.RawMessage{
		"echo": req.Input,
	}
	return json.Marshal(out)
}

func ForceErrorAction(_ context.Context, _ domain.RequestEnvelope, _ *domain.Session) ([]byte, error) {
	return nil, fmt.Errorf("forced action error")
}
