package registry

import (
	"context"
	"testing"

	"state-machine-engine/internal/domain"
)

func buildTestEvaluator() *Evaluator {
	e := NewEvaluator()
	e.Register("always", AlwaysCondition)
	e.Register("input_exists", InputExistsCondition)
	e.Register("input_equals", InputEqualsCondition)
	return e
}

func TestEvaluator_Evaluate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		condition string
		input     []byte
		want      bool
		wantErr   bool
	}{
		{
			name:      "empty condition defaults true",
			condition: "",
			input:     nil,
			want:      true,
			wantErr:   false,
		},
		{
			name:      "always",
			condition: "always",
			input:     nil,
			want:      true,
			wantErr:   false,
		},
		{
			name:      "always malformed with args",
			condition: "always:oops",
			input:     nil,
			want:      false,
			wantErr:   true,
		},
		{
			name:      "input exists true",
			condition: "input_exists:userId",
			input:     []byte(`{"userId":"123"}`),
			want:      true,
			wantErr:   false,
		},
		{
			name:      "input exists false",
			condition: "input_exists:userId",
			input:     []byte(`{"other":"123"}`),
			want:      false,
			wantErr:   false,
		},
		{
			name:      "input exists malformed",
			condition: "input_exists:",
			input:     []byte(`{"userId":"123"}`),
			want:      false,
			wantErr:   true,
		},
		{
			name:      "input equals true",
			condition: "input_equals:status:approved",
			input:     []byte(`{"status":"approved"}`),
			want:      true,
			wantErr:   false,
		},
		{
			name:      "input equals false value mismatch",
			condition: "input_equals:status:approved",
			input:     []byte(`{"status":"rejected"}`),
			want:      false,
			wantErr:   false,
		},
		{
			name:      "input equals false key missing",
			condition: "input_equals:status:approved",
			input:     []byte(`{"other":"approved"}`),
			want:      false,
			wantErr:   false,
		},
		{
			name:      "input equals malformed",
			condition: "input_equals:status",
			input:     []byte(`{"status":"approved"}`),
			want:      false,
			wantErr:   true,
		},
		{
			name:      "unknown condition",
			condition: "does_not_exist:something",
			input:     []byte(`{"a":"b"}`),
			want:      false,
			wantErr:   true,
		},
		{
			name:      "invalid input json",
			condition: "input_exists:userId",
			input:     []byte(`{"userId":`),
			want:      false,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			e := buildTestEvaluator()
			req := domain.RequestEnvelope{
				SessionID: "s-1",
				Input:     tt.input,
			}

			got, err := e.Evaluate(context.Background(), tt.condition, req, &domain.Session{ID: "s-1"})
			if (err != nil) != tt.wantErr {
				t.Fatalf("error mismatch: gotErr=%v wantErr=%v err=%v", err != nil, tt.wantErr, err)
			}
			if got != tt.want {
				t.Fatalf("result mismatch: got=%v want=%v", got, tt.want)
			}
		})
	}
}
