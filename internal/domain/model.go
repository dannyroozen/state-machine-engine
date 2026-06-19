package domain

import (
	"encoding/json"
	"time"
)

const (
	MaxRequestSizeBytes = 1 << 20 // 1 MB
	ExitState           = "exit"
)

// RequestEnvelope is the normalized input contract for CLI and future REST.
type RequestEnvelope struct {
	SessionID string          `json:"session_id"`
	Input     json.RawMessage `json:"input"`
}

// ResponseEnvelope is the normalized output contract for CLI and future REST.
type ResponseEnvelope struct {
	SessionID string          `json:"session_id"`
	State     string          `json:"state"`
	Output    json.RawMessage `json:"output,omitempty"`
	Error     *ErrorBlock     `json:"error,omitempty"`
}

type ErrorBlock struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Session is storage-agnostic and serializable.
type Session struct {
	ID        string          `json:"id"`
	State     string          `json:"state"`
	Data      json.RawMessage `json:"data,omitempty"`
	ExpiresAt time.Time       `json:"expires_at"`
}

type RuntimeConfig struct {
	Session   SessionRuntimeConfig   `json:"session"`
	Assistant AssistantRuntimeConfig `json:"assistant,omitempty"`
}

type SessionRuntimeConfig struct {
	TTL          time.Duration `json:"ttl"`
	FileLocation string        `json:"file_location,omitempty"`
}

type AssistantRuntimeConfig struct {
	MaxTurns   int                 `json:"max_turns,omitempty"`
	TargetDir  string              `json:"target_dir,omitempty"`
	ArchiveDir string              `json:"archive_dir,omitempty"`
	Ollama     OllamaRuntimeConfig `json:"ollama,omitempty"`
}

type OllamaRuntimeConfig struct {
	BaseURL        string `json:"base_url,omitempty"`
	Model          string `json:"model,omitempty"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}

// StateMachine definition loaded from configuration.
type StateMachine struct {
	Name       string                     `json:"name" yaml:"Name"`
	Initial    string                     `json:"initial_state" yaml:"InitialState"`
	ErrorState string                     `json:"error_state" yaml:"ErrorState"`
	States     map[string]StateDefinition `json:"states" yaml:"States"`
	// TODO: Should we add asynchronous Events?
}

// StateDefinition defines the transitions between states
type StateDefinition struct {
	Action      string                 `json:"action,omitempty" yaml:"Action,omitempty"` // optional plugin action name
	Transitions []TransitionDefinition `json:"transitions" yaml:"Transitions"`
}

type TransitionDefinition struct {
	ID        string `json:"id" yaml:"ID"`
	Condition string `json:"condition,omitempty" yaml:"Condition,omitempty"` // evaluated by condition evaluator
	Action    string `json:"action,omitempty" yaml:"Action,omitempty"`       // optional plugin action name
	Target    string `json:"target" yaml:"Target"`
}
