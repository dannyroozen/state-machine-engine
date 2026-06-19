package json

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestProvider_LoadStateMachine_Success(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	machinePath := filepath.Join(dir, "machine.json")
	runtimePath := filepath.Join(dir, "runtime.json")

	if err := os.WriteFile(machinePath, []byte(`{
		"name":"m1",
		"initial_state":"start",
		"error_state":"error",
		"states":{"start":{"transitions":[{"id":"t1","target":"exit"}]},"error":{"transitions":[{"id":"te","target":"exit"}]},"exit":{"transitions":[]}}
	}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(runtimePath, []byte(`{"session":{"ttl":"30s"}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	p := NewProvider(machinePath, runtimePath)

	m, err := p.LoadStateMachine(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Name != "m1" || m.Initial != "start" || m.ErrorState != "error" {
		t.Fatalf("unexpected machine: %+v", m)
	}
}

func TestProvider_LoadStateMachine_Errors(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	p := NewProvider(filepath.Join(dir, "missing.json"), filepath.Join(dir, "runtime.json"))

	_, err := p.LoadStateMachine(context.Background())
	if err == nil || !strings.Contains(err.Error(), "unable to read file") {
		t.Fatalf("expected file read error, got %v", err)
	}

	badMachine := filepath.Join(dir, "bad-machine.json")
	if err = os.WriteFile(badMachine, []byte(`{"name":`), 0o600); err != nil {
		t.Fatal(err)
	}
	p = NewProvider(badMachine, filepath.Join(dir, "runtime.json"))

	_, err = p.LoadStateMachine(context.Background())
	if err == nil || !strings.Contains(err.Error(), "invalid machine json") {
		t.Fatalf("expected invalid machine json error, got %v", err)
	}
}

func TestProvider_LoadRuntimeConfig_SuccessAndErrors(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	machinePath := filepath.Join(dir, "machine.json")
	runtimePath := filepath.Join(dir, "runtime.json")
	if err := os.WriteFile(machinePath, []byte(`{"name":"m1","initial_state":"s","error_state":"e","states":{"s":{"transitions":[{"id":"t","target":"exit"}]},"e":{"transitions":[{"id":"te","target":"exit"}]},"exit":{"transitions":[]}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(runtimePath, []byte(`{
		"session":{"ttl":"2m","file_location":"target/sessions"},
		"assistant":{"max_turns":55,"ollama":{"base_url":"http://localhost:11434","model":"llama3.1","timeout_seconds":30}}
	}`), 0o600); err != nil {
		t.Fatal(err)
	}

	p := NewProvider(machinePath, runtimePath)
	rt, err := p.LoadRuntimeConfig(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rt.Session.TTL != 2*time.Minute || rt.Assistant.Ollama.Model != "llama3.1" || rt.Assistant.MaxTurns != 55 {
		t.Fatalf("unexpected ttl: %v", rt.Session.TTL)
	}

	badRuntime := filepath.Join(dir, "bad-runtime.json")
	if err = os.WriteFile(badRuntime, []byte(`{"session_ttl":"not-a-duration"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	p = NewProvider(machinePath, badRuntime)
	_, err = p.LoadRuntimeConfig(context.Background())
	if err == nil || !strings.Contains(err.Error(), "invalid session_ttl") {
		t.Fatalf("expected invalid session_ttl error, got %v", err)
	}
}
