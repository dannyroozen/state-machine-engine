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

// Register a condition for an application's state machine
// Thread-safe
func (e *Evaluator) Register(name string, fn ConditionFunc) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.conditions[name] = fn
}

// Evaluate a condition for true/false.
// Thread-safe
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

// AlwaysCondition to always return true.
func AlwaysCondition(_ context.Context, args string, _ domain.RequestEnvelope, _ *domain.Session) (bool, error) {
	if strings.TrimSpace(args) != "" {
		return false, errors.New("invalid request: malformed always condition")
	}
	return true, nil
}

// InputExistsCondition evaluates whether a value for a given key in the input exists.
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

// InputEqualsCondition evaluates whether the input value for a given key matches the provided expression.
// (e.g. input_equals:foo=bar matches -- input '{"session_id": "123", "input": {"foo": "bar"}}')
func InputEqualsCondition(_ context.Context, args string, req domain.RequestEnvelope, _ *domain.Session) (bool, error) {
	expr := strings.TrimSpace(args)
	if expr == "" {
		return false, errors.New("invalid request: malformed input_equals condition")
	}

	input, err := parseInput(req)
	if err != nil {
		return false, err
	}

	return evalInputEqualsExpr(expr, input)
}

// evalInputEqualsExpr is the meat of the InputEqualsCondition
// and exists as a separate function to be able to be called recursively.
// || and && operations can be included in an input_equals expression, broken down then into their own expressions.
func evalInputEqualsExpr(expr string, input map[string]any) (bool, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return false, errors.New("invalid request: malformed input_equals condition")
	}

	// OR has lower precedence than AND, so split OR first and evaluate each part recursively.
	orParts := splitLogicalParts(expr, "||")
	if len(orParts) > 1 {
		for _, part := range orParts {
			ok, err := evalInputEqualsExpr(part, input)
			if err != nil {
				return false, err
			}
			if ok {
				// First OR expression that is true wins, even if the expression is malformed on later parts.
				// TODO: Do we want to enforce that the whole expression is well-formed?
				// TODO: Should we extend the validator to validate that all of the conditions match up to registered conditions and are well-formed?
				return true, nil
			}
		}
		return false, nil
	}

	andParts := splitLogicalParts(expr, "&&")
	if len(andParts) > 1 {
		for _, part := range andParts {
			ok, err := evalInputEqualsExpr(part, input)
			if err != nil {
				return false, err
			}
			if !ok {
				// Every part of an AND expression must be true. Return early when we find an expression that is false.
				return false, nil
			}
		}
		return true, nil
	}

	return evalInputEqualsClause(expr, input)
}

func splitLogicalParts(expr, op string) []string {
	raw := strings.Split(expr, op)
	parts := make([]string, 0, len(raw))
	for _, p := range raw {
		parts = append(parts, strings.TrimSpace(p))
	}
	return parts
}

// evalInputEqualsClause evaluates just the variable equals portion of the expression.
// (e.g. input:state=accepted for "input": {"status": "accepted"})
func evalInputEqualsClause(clause string, input map[string]any) (bool, error) {
	clause = strings.TrimSpace(clause)
	if clause == "" {
		return false, errors.New("invalid request: malformed input_equals condition")
	}

	// Optional prefixes so these are both valid:
	// - status=accepted
	// - input:status=accepted
	// - input_equals:status=accepted
	switch {
	case strings.HasPrefix(clause, "input_equals:"):
		clause = strings.TrimSpace(strings.TrimPrefix(clause, "input_equals:"))
	case strings.HasPrefix(clause, "input:"):
		clause = strings.TrimSpace(strings.TrimPrefix(clause, "input:"))
	}

	parts := strings.SplitN(clause, "=", 2)
	if len(parts) != 2 {
		return false, errors.New("invalid request: malformed input_equals condition")
	}

	key := strings.TrimSpace(parts[0])      // white space is ok
	expected := strings.TrimSpace(parts[1]) // white space is ok
	if key == "" {
		return false, errors.New("invalid request: malformed input_equals condition")
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
