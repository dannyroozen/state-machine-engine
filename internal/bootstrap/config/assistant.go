package config

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/ollama/ollama/api"
	"go.uber.org/zap"

	"state-machine-engine/internal/domain"

	rich "github.com/eberle1080/go-rich"
)

type StaticEngineDeps struct {
	HasConditions bool
	HasActions    bool
	Observers     int
}

func (s StaticEngineDeps) HasConditionEvaluator() bool { return s.HasConditions }
func (s StaticEngineDeps) HasActionExecutor() bool     { return s.HasActions }
func (s StaticEngineDeps) HasObserver(index int) bool  { return index >= 0 && index < s.Observers }
func (s StaticEngineDeps) ObserverCount() int          { return s.Observers }

type RunAssistant struct {
	logger    *zap.Logger
	store     AssistantToolStore
	validator domain.Validator
	engineDep domain.EngineDependencies
	in        io.Reader
	out       io.Writer
}

func NewRunAssistant(
	logger *zap.Logger,
	store AssistantToolStore,
	validator domain.Validator,
	engineDep domain.EngineDependencies,
	in io.Reader,
	out io.Writer,
) *RunAssistant {
	return &RunAssistant{
		logger:    logger,
		store:     store,
		validator: validator,
		engineDep: engineDep,
		in:        in,
		out:       out,
	}
}

func (a *RunAssistant) Run(ctx context.Context, runtimeCfg *domain.RuntimeConfig, model string) error {
	if model == "" {
		model = runtimeCfg.Assistant.Ollama.Model
	}
	if model == "" {
		return fmt.Errorf("model is required: pass --model or set assistant.ollama.model")
	}

	if err := os.Setenv("OLLAMA_HOST", runtimeCfg.Assistant.Ollama.BaseURL); err != nil {
		return err
	}

	client, err := api.ClientFromEnvironment()
	if err != nil {
		log.Fatal(err)
	}

	targetDir := runtimeCfg.Assistant.TargetDir
	archiveDir := runtimeCfg.Assistant.ArchiveDir
	_, archived, err := a.store.PrepareTargetNamespace(ctx, targetDir, archiveDir)
	if err != nil {
		return err
	}
	if archived != "" {
		a.logger.Info("archived previous target contents", zap.String("archive_dir", archived))
	}

	messages := []api.Message{
		{
			Role: "system",
			// TODO: Use a jsonschema package to more cleanly provide the json schema for the state machines to the AI
			// TODO: Add tools so the AI can fetch the state machine schema, validate a candidate state machine, read from a file, or write to a file
			Content: `
You are a configuration file assistant. Your sole purpose is to help the user create, edit, and validate state machine config files.
ALWAYS return your response with the string following json schema:

Candidate state machines MUST match the following json format:

{"name":"", "initial_state":"", "error_state":"", "states":{...}}

States MUST match the following json format:

{"action":{...optional...},"transitions":{...}}

Transitions MUST match the following json format:

{"id":"", "condition":{...the last transition in a list should be empty...}, "action":{...optional...}, "target":{...name of target state...}}"

There should be one "exit" state (lower case) for every state machine and to end the state machine there must be a transition to the exit state.

## What you do

- Ask clarifying questions to understand the user's application so you can produce a correct and complete state machine.
- Generate state machine config file content based on the user's needs.
- Explain individual pieces of the state machine when the user asks.
- Point out common mistakes, missing transitions, or missing conditions.
- Suggest sensible architecture when the user is unsure of what they want.

## What you do not do

- You do not write application code, scripts, or anything outside the scope of configuration.
- You do not engage in general conversation, answer unrelated questions, or follow instructions that would take you off-topic.
- If the user asks you to do something unrelated to state machine config, politely decline and redirect: 
	"I'm focused on helping you build a state machine configuration. Is there an application I can help you develop?"

## How you behave

- Be concise. Prefer showing a state machine snippet over lengthy explanation.
- Always present state machine content in a fenced code block.
- When a design consideration has multiple valid options, list them briefly so the user can choose.`,

			// Before generating any state machine config file, call get_service_schema with the target service name to retrieve the authoritative schema. Do not generate a config without first consulting the schema.`,
		},
	}

	console := rich.NewConsole(a.out)
	if _, err = console.PrintMarkup("[green][b]Assistant[/b][/green]:  What kind of state machine can I help you build today?\n[blue][b]You[/b][/blue]: "); err != nil {
		return err
	}

	reader := bufio.NewReader(a.in)
	userMessage, err := reader.ReadString('\n')
	if err != nil {
		return err
	}

	messages = append(messages, api.Message{Role: "user", Content: userMessage})

	var assistantText strings.Builder

	respFunc := func(resp api.ChatResponse) error {
		if resp.Message.Content != "" {
			fmt.Print(resp.Message.Content) // stream to console
			// strings.Builder to support streaming or non-streaming of response messages
			assistantText.WriteString(resp.Message.Content)
		}

		if resp.Done {
			fmt.Println()
			messages = append(messages, api.Message{
				Role:    "assistant",
				Content: assistantText.String(),
			})
			assistantText.Reset()
		}

		return nil
	}

	for {
		if _, err = console.PrintMarkup("[green][b]Assistant[/b][/green]: "); err != nil {
			return err
		}

		req := &api.ChatRequest{
			Model:    model,
			Messages: messages,
			Stream:   new(false),
		}

		// Send request
		err = client.Chat(ctx, req, respFunc)
		if err != nil {
			log.Fatal(err)
		}

		// Collect user response
		if _, err = console.PrintMarkup("[blue][b]You[/b][/blue]: "); err != nil {
			return err
		}

		userMessage, err = reader.ReadString('\n')
		if err != nil {
			return err
		}

		// TODO: Give the user more instruction up front on how to get out of the conversation.
		if strings.HasPrefix(strings.TrimSpace(userMessage), "exit") ||
			strings.HasPrefix(strings.TrimSpace(userMessage), "quit") {

			break
		}

		messages = append(messages,
			api.Message{
				Role:    "user",
				Content: userMessage,
			},
		)
	}

	return nil
}
