package simple

import (
	"context"
	"strings"
	"testing"

	"state-machine-engine/internal/domain"
)

func TestEvaluator_Evaluate(t *testing.T) {
	t.Parallel()

	e := NewEvaluator()
	req := func(input string) domain.RequestEnvelope {
		return domain.RequestEnvelope{SessionID: "s1", Input: []byte(input)}
	}

	tests := []struct {
		name      string
		condition string
		req       domain.RequestEnvelope
		want      bool
		wantErr   bool
	}{
		{"empty", "", req(`{}`), true, false},
		{"always", "always", req(`{}`), true, false},
		{"input_exists true", "input_exists:x", req(`{"x":1}`), true, false},
		{"input_exists false", "input_exists:x", req(`{"y":1}`), false, false},
		{"input_equals true", "input_equals:status:ok", req(`{"status":"ok"}`), true, false},
		{"input_equals false", "input_equals:status:ok", req(`{"status":"bad"}`), false, false},
		{"input_equals malformed", "input_equals:status", req(`{"status":"ok"}`), false, true},
		{"invalid input json", "input_exists:x", req(`{"x":`), false, true},
		{"unsupported", "other:thing", req(`{}`), false, true},
		{"trimmed condition", "  always  ", req(`{}`), true, false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := e.Evaluate(context.Background(), tt.condition, tt.req, &domain.Session{ID: "s1"})
			if (err != nil) != tt.wantErr {
				t.Fatalf("err mismatch got=%v wantErr=%v err=%v", err != nil, tt.wantErr, err)
			}
			if tt.wantErr && err != nil && !strings.Contains(err.Error(), "invalid request") {
				t.Fatalf("expected invalid request wrapping, got %v", err)
			}
			if got != tt.want {
				t.Fatalf("result mismatch got=%v want=%v", got, tt.want)
			}
		})
	}
}
