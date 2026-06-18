package registry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"state-machine-engine/internal/domain"
)

type ConditionFunc func(ctx context.Context, args string, req domain.RequestEnvelope, session *domain.Session) (bool, error)

type Evaluator struct {
	mu         sync.RWMutex
	conditions map[string]ConditionFunc
}

func NewEvaluator() *Evaluator {
	return &Evaluator{
		conditions: make(map[string]ConditionFunc),
	}
}

func (e *Evaluator) Register(name string, fn ConditionFunc) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.conditions[name] = fn
}

func (e *Evaluator) Evaluate(ctx context.Context, condition string, req domain.RequestEnvelope, session *domain.Session) (bool, error) {
	condition = strings.TrimSpace(condition)
	if condition == "" {
		return true, nil
	}

	name, args := splitCondition(condition)

	e.mu.RLock()
	fn, ok := e.conditions[name]
	e.mu.RUnlock()
	if !ok {
		return false, fmt.Errorf("invalid request: unsupported condition %q", condition)
	}

	return fn(ctx, args, req, session)
}

func splitCondition(raw string) (name, args string) {
	parts := strings.SplitN(raw, ":", 2)
	name = strings.TrimSpace(parts[0])
	if len(parts) == 1 {
		return name, ""
	}
	return name, strings.TrimSpace(parts[1])
}

// Built-in condition functions for this registry implementation.

func AlwaysCondition(_ context.Context, args string, _ domain.RequestEnvelope, _ *domain.Session) (bool, error) {
	if strings.TrimSpace(args) != "" {
		return false, errors.New("invalid request: malformed always condition")
	}
	return true, nil
}

func InputExistsCondition(_ context.Context, args string, req domain.RequestEnvelope, _ *domain.Session) (bool, error) {
	key := strings.TrimSpace(args)
	if key == "" {
		return false, errors.New("invalid request: malformed input_exists condition")
	}

	input, err := parseInput(req)
	if err != nil {
		return false, err
	}

	_, ok := input[key]
	return ok, nil
}

func InputEqualsCondition(_ context.Context, args string, req domain.RequestEnvelope, _ *domain.Session) (bool, error) {
	parts := strings.SplitN(args, ":", 2)
	if len(parts) != 2 {
		return false, errors.New("invalid request: malformed input_equals condition")
	}

	key := strings.TrimSpace(parts[0])
	expected := strings.TrimSpace(parts[1])
	if key == "" {
		return false, errors.New("invalid request: malformed input_equals condition")
	}

	input, err := parseInput(req)
	if err != nil {
		return false, err
	}

	val, ok := input[key]
	if !ok {
		return false, nil
	}
	return fmt.Sprintf("%v", val) == expected, nil
}

func parseInput(req domain.RequestEnvelope) (map[string]any, error) {
	if len(req.Input) == 0 {
		return map[string]any{}, nil
	}

	var input map[string]any
	if err := json.Unmarshal(req.Input, &input); err != nil {
		return nil, fmt.Errorf("invalid request: invalid input object: %w", err)
	}
	return input, nil
}
