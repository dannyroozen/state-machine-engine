// This file contains a simple json-based configuration provider as an example.
// Other implementations for configuration storage could include a database, for example.

package json

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"state-machine-engine/internal/domain"
)

type Provider struct {
	machinePath string
	runtimePath string
}

func NewProvider(machinePath, runtimePath string) *Provider {
	return &Provider{
		machinePath: machinePath,
		runtimePath: runtimePath,
	}
}

type runtimeConfigDTO struct {
	Session struct {
		TTL          string `json:"ttl"`
		FileLocation string `json:"file_location"`
	} `json:"session"`
	Assistant struct {
		TargetDir  string `json:"target_dir"`
		ArchiveDir string `json:"archive_dir"`
		Ollama     struct {
			BaseURL        string `json:"base_url"`
			Model          string `json:"model"`
			TimeoutSeconds int    `json:"timeout_seconds"`
		} `json:"ollama"`
		MaxTurns int `json:"max_turns"`
	} `json:"assistant"`
}

func (p *Provider) LoadStateMachine(_ context.Context) (*domain.StateMachine, error) {
	raw, err := os.ReadFile(p.machinePath)
	if err != nil {
		return nil, fmt.Errorf("unable to read file: %v", err)
	}

	var machine domain.StateMachine
	if err := json.Unmarshal(raw, &machine); err != nil {
		return nil, fmt.Errorf("invalid machine json: %v", err)
	}
	return &machine, nil
}

func (p *Provider) LoadStateMachineRaw(_ context.Context) ([]byte, error) {
	raw, err := os.ReadFile(p.machinePath)
	if err != nil {
		return nil, fmt.Errorf("unable to read file: %v", err)
	}
	return raw, nil
}

func (p *Provider) LoadRuntimeConfig(_ context.Context) (*domain.RuntimeConfig, error) {
	raw, err := os.ReadFile(p.runtimePath)
	if err != nil {
		return nil, fmt.Errorf("unable to read file: %v", err)
	}

	var dto runtimeConfigDTO
	if err := json.Unmarshal(raw, &dto); err != nil {
		return nil, fmt.Errorf("invalid runtime json: %v", err)
	}

	ttl, err := time.ParseDuration(dto.Session.TTL)
	if err != nil {
		return nil, fmt.Errorf("invalid session_ttl: %v", err)
	}

	cfg := &domain.RuntimeConfig{
		Session: domain.SessionRuntimeConfig{
			TTL:          ttl,
			FileLocation: dto.Session.FileLocation,
		},
		Assistant: domain.AssistantRuntimeConfig{
			MaxTurns:   dto.Assistant.MaxTurns,
			TargetDir:  dto.Assistant.TargetDir,
			ArchiveDir: dto.Assistant.ArchiveDir,
			Ollama: domain.OllamaRuntimeConfig{
				BaseURL:        dto.Assistant.Ollama.BaseURL,
				Model:          dto.Assistant.Ollama.Model,
				TimeoutSeconds: dto.Assistant.Ollama.TimeoutSeconds,
			},
		},
	}

	if cfg.Session.FileLocation == "" {
		cfg.Session.FileLocation = "target/sessions"
	}
	if cfg.Assistant.TargetDir == "" {
		cfg.Assistant.TargetDir = "target/config-assistant"
	}
	if cfg.Assistant.ArchiveDir == "" {
		cfg.Assistant.ArchiveDir = filepath.Join(cfg.Assistant.TargetDir, "archive")
	}
	if cfg.Assistant.MaxTurns <= 0 {
		cfg.Assistant.MaxTurns = 40
	}
	if cfg.Assistant.Ollama.BaseURL == "" {
		cfg.Assistant.Ollama.BaseURL = "http://localhost:11434"
	}
	if cfg.Assistant.Ollama.TimeoutSeconds <= 0 {
		cfg.Assistant.Ollama.TimeoutSeconds = 120
	}

	return cfg, nil
}

func (p *Provider) PrepareTargetNamespace(_ context.Context, targetDir, archiveDir string) (string, string, error) {
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", "", fmt.Errorf("failed creating target dir: %v", err)
	}
	if err := os.MkdirAll(archiveDir, 0o755); err != nil {
		return "", "", fmt.Errorf("failed creating archive dir: %v", err)
	}

	entries, err := os.ReadDir(targetDir)
	if err != nil {
		return "", "", fmt.Errorf("failed reading target dir: %v", err)
	}

	archiveBatch := ""
	if len(entries) > 0 {
		archiveBatch = filepath.Join(archiveDir, time.Now().Format("20060102-150405"))
		if err := os.MkdirAll(archiveBatch, 0o755); err != nil {
			return "", "", fmt.Errorf("failed creating archive batch dir: %v", err)
		}
		for _, entry := range entries {
			if entry.Name() == "archive" {
				continue
			}
			src := filepath.Join(targetDir, entry.Name())
			dst := filepath.Join(archiveBatch, entry.Name())
			if err := os.Rename(src, dst); err != nil {
				return "", "", fmt.Errorf("failed archiving %s: %v", entry.Name(), err)
			}
		}
	}

	namespaceDir := filepath.Join(targetDir, "run-"+time.Now().Format("20060102-150405"))
	if err := os.MkdirAll(namespaceDir, 0o755); err != nil {
		return "", "", fmt.Errorf("failed creating namespace dir: %v", err)
	}
	return namespaceDir, archiveBatch, nil
}

func (p *Provider) WriteStateMachineAtPath(_ context.Context, machine *domain.StateMachine, outPath string) error {
	raw, err := json.MarshalIndent(machine, "", "  ")
	if err != nil {
		return fmt.Errorf("failed marshaling machine: %v", err)
	}
	if err := os.WriteFile(outPath, raw, 0o600); err != nil {
		return fmt.Errorf("failed writing machine: %v", err)
	}
	return nil
}
