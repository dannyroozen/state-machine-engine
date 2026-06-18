package config

import (
	"context"

	"state-machine-engine/internal/domain"
)

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type AssistantToolStore interface {
	domain.ConfigProvider
	LoadStateMachineRaw(ctx context.Context) ([]byte, error)
	PrepareTargetNamespace(ctx context.Context, targetDir, archiveDir string) (namespaceDir, archivedDir string, err error)
	WriteStateMachineAtPath(ctx context.Context, machine *domain.StateMachine, outPath string) error
}
