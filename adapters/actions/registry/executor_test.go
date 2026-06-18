package registry

import (
	"context"
	"strings"
	"testing"

	"state-machine-engine/internal/domain"
)

func TestExecutor_RegisterAndExecute(t *testing.T) {
	t.Parallel()

	exec := NewExecutor()
	exec.Register("ok", func(_ context.Context, _ domain.RequestEnvelope, _ *domain.Session, _ domain.TransitionEvent) ([]byte, error) {
		return []byte(`{"ok":true}`), nil
	})

	out, err := exec.Execute(context.Background(), "ok", domain.RequestEnvelope{}, &domain.Session{}, domain.TransitionEvent{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(out) != `{"ok":true}` {
		t.Fatalf("unexpected output: %s", string(out))
	}
}

func TestExecutor_UnknownAction(t *testing.T) {
	t.Parallel()

	exec := NewExecutor()
	_, err := exec.Execute(context.Background(), "missing", domain.RequestEnvelope{}, &domain.Session{}, domain.TransitionEvent{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), `unknown action "missing"`) {
		t.Fatalf("expected unknown action error, got: %v", err)
	}
}

func TestBuiltInActions(t *testing.T) {
	t.Parallel()

	if out, err := NoopAction(context.Background(), domain.RequestEnvelope{}, &domain.Session{}, domain.TransitionEvent{}); err != nil || out != nil {
		t.Fatalf("noop unexpected result out=%v err=%v", out, err)
	}

	echoOut, err := EchoInputAction(context.Background(), domain.RequestEnvelope{
		Input: []byte(`{"x":1}`),
	}, &domain.Session{}, domain.TransitionEvent{})
	if err != nil {
		t.Fatalf("echo unexpected error: %v", err)
	}
	if string(echoOut) != `{"echo":{"x":1}}` {
		t.Fatalf("unexpected echo output: %s", string(echoOut))
	}

	textOut, err := EchoInputTextAction(context.Background(), domain.RequestEnvelope{
		Input: []byte(`{"x":1}`),
	}, &domain.Session{}, domain.TransitionEvent{})
	if err != nil {
		t.Fatalf("text echo unexpected error: %v", err)
	}
	if !strings.Contains(string(textOut), `{"x":1}`) {
		t.Fatalf("unexpected text output: %s", string(textOut))
	}

	binOut, err := EmitBinaryAction(context.Background(), domain.RequestEnvelope{}, &domain.Session{}, domain.TransitionEvent{})
	if err != nil {
		t.Fatalf("binary output unexpected error: %v", err)
	}
	if len(binOut) != 4 {
		t.Fatalf("unexpected binary output length: %d", len(binOut))
	}

	if _, err = ForceErrorAction(context.Background(), domain.RequestEnvelope{}, &domain.Session{}, domain.TransitionEvent{}); err == nil {
		t.Fatal("expected forced error")
	}
}
