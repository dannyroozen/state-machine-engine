package simple

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"state-machine-engine/internal/domain"
)

type Evaluator struct{}

func NewEvaluator() *Evaluator {
	return &Evaluator{}
}

// Evaluate will parse a condition in a configured state machine and produce a boolean indicating condition pass or fail
// Supported conditions:
// - "always"
// - "input_exists:<key>"
// - "input_equals:<key>:<value>"
// TODO: Implement a conditions plugin that could be a little more generic/extendible, like the actions registry
func (e *Evaluator) Evaluate(_ context.Context, condition string, req domain.RequestEnvelope, _ *domain.Session) (bool, error) {
	condition = strings.TrimSpace(condition)
	if condition == "" { // empty condition is assumed always true, there's nothing evaluating to true/false that can prevent it
		return true, nil
	}
	if condition == "always" {
		return true, nil
	}

	var input map[string]any
	if len(req.Input) > 0 {
		if err := json.Unmarshal(req.Input, &input); err != nil {
			return false, fmt.Errorf("invalid request: invalid input object: %v", err)
		}
	} else {
		input = map[string]any{}
	}

	if strings.HasPrefix(condition, "input_exists:") {
		key := strings.TrimPrefix(condition, "input_exists:")
		_, ok := input[key]
		return ok, nil
	}

	if strings.HasPrefix(condition, "input_equals:") {
		rest := strings.TrimPrefix(condition, "input_equals:")
		parts := strings.SplitN(rest, ":", 2)
		if len(parts) != 2 {
			return false, errors.New("invalid request: malformed input_equals condition")
		}
		key, expected := parts[0], parts[1]
		val, ok := input[key]
		if !ok {
			return false, nil
		}
		return fmt.Sprintf("%v", val) == expected, nil
	}

	return false, fmt.Errorf("invalid request: unsupported condition %q", condition)
}
