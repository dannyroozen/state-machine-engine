package logging

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"state-machine-engine/internal/domain"
)

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, context.DeadlineExceeded }

func TestObserver_OnTransition_WritesJSONLine(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	obs := NewObserver(&buf)

	err := obs.OnTransition(context.Background(), domain.TransitionEvent{
		SessionID:   "s1",
		MachineName: "m1",
		FromState:   "a",
		Transition:  "t1",
		ToState:     "b",
		Success:     true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, `"session_id":"s1"`) || !strings.HasSuffix(out, "\n") {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestObserver_OnTransition_WriteError(t *testing.T) {
	t.Parallel()

	obs := NewObserver(failWriter{})
	err := obs.OnTransition(context.Background(), domain.TransitionEvent{})
	if err == nil {
		t.Fatal("expected write error")
	}
}
