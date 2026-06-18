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
	exec.Register("ok", func(_ context.Context, _ domain.RequestEnvelope, _ *domain.Session) ([]byte, error) {
		return []byte(`{"ok":true}`), nil
	})

	out, err := exec.Execute(context.Background(), "ok", domain.RequestEnvelope{}, &domain.Session{})
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
	_, err := exec.Execute(context.Background(), "missing", domain.RequestEnvelope{}, &domain.Session{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), `unknown action "missing"`) {
		t.Fatalf("expected ErrActionFailed, got: %v", err)
	}
}

func TestBuiltInActions(t *testing.T) {
	t.Parallel()

	if out, err := NoopAction(context.Background(), domain.RequestEnvelope{}, &domain.Session{}); err != nil || out != nil {
		t.Fatalf("noop unexpected result out=%v err=%v", out, err)
	}

	echoOut, err := EchoInputAction(context.Background(), domain.RequestEnvelope{
		Input: []byte(`{"x":1}`),
	}, &domain.Session{})
	if err != nil {
		t.Fatalf("echo unexpected error: %v", err)
	}
	if string(echoOut) != `{"echo":{"x":1}}` {
		t.Fatalf("unexpected echo output: %s", string(echoOut))
	}

	if _, err = ForceErrorAction(context.Background(), domain.RequestEnvelope{}, &domain.Session{}); err == nil {
		t.Fatal("expected forced error")
	}
}
