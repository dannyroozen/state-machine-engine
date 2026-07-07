package cmd

import "state-machine-engine/internal/app"

type Runtime interface {
	BuildService(machinePath, runtimePath string) *app.Service
	RunRequest(socketPath, input, endpoint string) error
	RunConfigAssistant(machinePath, runtimePath, model string) error
}
