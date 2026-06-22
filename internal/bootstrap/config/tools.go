package config

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"state-machine-engine/internal/domain"
)

const writeStateMachineToFileToolName = "WriteStateMachineToFile"

type assistantToolEnvelope struct {
	Tool      string          `json:"tool"`
	Arguments json.RawMessage `json:"arguments"`
}

type writeStateMachineToFileArgs struct {
	StateMachine domain.StateMachine `json:"state_machine"`
}

func writeToolPrompt() string {
	return `
You have one tool available:

Tool name: WriteStateMachineToFile
Purpose: Persist the final candidate state machine into the prepared target namespace directory.

When you want to call the tool, your response MUST be ONLY valid JSON in this exact shape:
{
  "tool": "WriteStateMachineToFile",
  "arguments": {
    "state_machine": { ... full candidate state machine ... }
  }
}

Do not include markdown fences when calling a tool.
Only call this tool after the user confirms they are ready to save.
`
}

func maybeRunAssistantTool(
	ctx context.Context,
	store AssistantToolStore,
	namespaceDir string,
	assistantMessage string,
) (handled bool, toolResult string, err error) {
	logger.Debug(fmt.Sprintf("maybeRunAssistantTool: %s", assistantMessage))
	trimmed := strings.TrimSpace(assistantMessage)
	if trimmed == "" {
		return false, "", nil
	}

	var envelope assistantToolEnvelope
	if err = json.Unmarshal([]byte(trimmed), &envelope); err != nil {
		return false, "", nil
	}

	if envelope.Tool != writeStateMachineToFileToolName {
		logger.Debug(fmt.Sprintf("maybeRunAssistantTool: ignoring tool %s", envelope.Tool))
		return false, "", nil
	}

	var args writeStateMachineToFileArgs
	if err = json.Unmarshal(envelope.Arguments, &args); err != nil {
		return true, "", fmt.Errorf("invalid tool arguments for %s: %w", writeStateMachineToFileToolName, err)
	}

	logger.Debug(fmt.Sprintf("maybeRunAssistantTool: calling tool %s", envelope.Tool))
	outPath, err := store.WriteStateMachineToFile(ctx, &args.StateMachine, namespaceDir)
	if err != nil {
		return true, "", err
	}

	return true, fmt.Sprintf("tool %s executed successfully; file written to %s", writeStateMachineToFileToolName, outPath), nil
}
