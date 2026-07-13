package json

import (
	"context"
	stdjson "encoding/json"
	"os"
	"path/filepath"
	"state-machine-engine/internal/domain"
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

func TestProvider_WriteStateMachineToFile_Success(t *testing.T) {
	t.Parallel()

	p := NewProvider("", "")
	targetDir := t.TempDir()

	machine := &domain.StateMachine{
		Name:       "order flow",
		Initial:    "start",
		ErrorState: "error",
		States: map[string]domain.StateDefinition{
			"start": {Transitions: []domain.TransitionDefinition{{ID: "t1", Target: "exit"}}},
			"error": {Transitions: []domain.TransitionDefinition{{ID: "te", Target: "exit"}}},
			"exit":  {},
		},
	}

	outPath, err := p.WriteStateMachineToFile(context.Background(), machine, targetDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(outPath, targetDir) {
		t.Fatalf("expected output in target dir, got %s", outPath)
	}
	if _, statErr := os.Stat(outPath); statErr != nil {
		t.Fatalf("expected file to exist: %v", statErr)
	}

	raw, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed reading output file: %v", err)
	}

	var saved domain.StateMachine
	if err = stdjson.Unmarshal(raw, &saved); err != nil {
		t.Fatalf("saved file is not valid json: %v", err)
	}
	if saved.Name != "order flow" || saved.Initial != "start" || saved.ErrorState != "error" {
		t.Fatalf("unexpected saved machine: %+v", saved)
	}
}

func TestProvider_GetSchema_Success(t *testing.T) {
	t.Parallel()

	p := NewProvider("", "")

	raw, err := p.GetSchema(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(raw) == 0 {
		t.Fatal("expected non-empty schema")
	}

	var schema map[string]any
	if err = stdjson.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("schema must be valid json: %v", err)
	}

	if _, ok := schema["$defs"].(map[string]any); !ok {
		t.Fatalf("expected schema to contain $defs")
	}

	if _, ok := schema["$defs"].(map[string]any)["StateMachine"].(map[string]any); !ok {
		t.Fatalf("expected schema to contain $defs.StateMachine")
	}

	properties, ok := schema["$defs"].(map[string]any)["StateMachine"].(map[string]any)["properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected schema to contain $defs.StateMachine.properties")
	}

	for _, key := range []string{"name", "initial_state", "error_state", "states"} {
		if _, found := properties[key]; !found {
			t.Fatalf("expected schema property %q", key)
		}
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
		"assistant":{"max_turns":55,"ollama":{"base_url":"http://localhost:11434","model":"llama3.1","timeout_seconds":30}},
		"engine":{"max_auto_advance_steps":250}
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
