# state-machine-engine

A pluggable Go state machine engine built around a stable core and replaceable adapters, with a Cobra/Viper CLI.

## Why state-machine-engine?

State Machines are simple in concept, but can be either simple or complex in implementation.
Typically, though, they are used as a code design paradigm to abstract a more complicated execution.
For example, rather than a series of nested if/then statements that gets bloated and complex far too fast, you
can define the various "states" of your application. To turn a light switch on or off, you could:

```go
func toggleLightSwitch(existingState boolean) (newState boolean) {
    if (existingState) { // light switch is on
        newState = false // turn it off
    } else {
        newState = true // turn it on
    }
    return
}
```

For a single boolean the implementation is dead simple and can be simplified even further to a single line.
But imagine a kanban board with multiple different possible states for a ticket to exist in and strict rules on
which states can come next and what needs to happen before they do. Or, potentially more or less complex, an IVR
call flow diagram where each state defines the prompts played to the caller and how they can respond.

So the state machine design paradigm ends up getting implemented and coded as such:

```go
    fsm.transitions[Open] = map[Event]State{
        StartProgress: InProgress,
    }
    fsm.transitions[InProgress] = map[Event]State{
        Close: Closed,
    }
    fsm.transitions[Closed] = map[Event]State{
        Reopen: Open,
    }

    fsm.actions[Open] = map[Event]Action{
        StartProgress: func() { fmt.Println("Ticket is now in progress") },
    }
    fsm.actions[InProgress] = map[Event]Action{
        Close: func() { fmt.Println("Ticket is now closed") },
    }
    fsm.actions[Closed] = map[Event]Action{
        Reopen: func() { fmt.Println("Ticket is reopened") },
    }
```

And this does simplify the problem so that the execution of the application can be better understood and maintained,
but now you are locked into an interface, defined in the coding language originally chosen by the first builder of
your state machine. For different applications, the state machine needs to be re-implemented, the logic of conditions,
actions on transitions, observers, and such re-coded for every implementation of your state machine.

But what if you could define your state machine once in a common data delivery format, such as JSON, YAML, 
or the W3C SCXML standard and run it against the same state machine engine regardless of its application? 
And if you need custom conditions, observers, or even custom memory storage or custom state machine execution interface? 
What if the underlying state machine was highly extensible and any one or all of those could work interchangeably 
with the underlying state machine engine?

## What this project is

This project separates **state machine orchestration** from **runtime integrations**:

- The **core engine** (`internal/`) handles deterministic transitions and lifecycle flow.
- The **adapters** (`adapters/`) provide concrete implementations for config, session storage, conditions, actions, observers, and transport.
- The **CLI layer** (`cmd/`) handles command parsing/config loading and delegates runtime behavior through an injectable `Runtime` interface.

This design keeps machine execution stable while making deployment/runtime integrations easy to customize.

---

## Architecture

## Core (`internal/`)

The core defines:

- Domain models (`StateMachine`, `Session`, request/response envelopes, transition events)
- Contracts/interfaces:
    - `ConfigProvider`
    - `SessionStore`
    - `ConditionEvaluator`
    - `ActionExecutor`
    - `Observer`
    - `Validator`
- Engine execution logic (`internal/engine`)
- Service orchestration (`internal/app`) that:
    1. Loads machine/runtime config
    2. Validates machine definition
    3. Loads/creates session
    4. Executes one engine step
    5. Persists session and returns response

In short: **`internal` is the framework/engine**.

## Adapters (`adapters/`)

Example implementations of those contracts:

- `adapters/config/file`: loads machine/runtime JSON from files
- `adapters/session/memory`: in-memory session store
- `adapters/conditions/registry`: condition registry with built-in conditions
- `adapters/conditions/simple`: simple condition evaluator, three conditions implemented in one method
- `adapters/actions/registry`: action registry with built-in actions
- `adapters/observers/logging`: transition-event logging observer
- `adapters/servers`: Unix socket server/client transport

In short: **`adapters` are replaceable plugins**.

## CLI (`cmd/`)

- Uses **Cobra** for command structure.
- Uses **Viper** for flags/env/config-file value binding.
- Delegates behavior through `Runtime` interface so runtime composition stays customizable.

---

## Key difference: engine vs plugins

| Aspect | `internal` | `adapters` |
|---|---|---|
| Responsibility | State machine semantics and orchestration | Concrete runtime integrations |
| Stability | Should change infrequently | Often customized per app/environment |
| Dependency direction | Depends on interfaces/contracts | Implements interfaces/contracts |
| Examples | Step execution, validation flow, domain errors | File config, memory store, registries, logging observer, Unix transport |

Mental model:

- **`internal` answers:** “How does the machine run?”
- **`adapters` answer:** “Where do config/sessions come from and what conditions/actions/IO do we plug in?”

---

## Configuration files

Defaults:

- `config/machine.json` — state machine definition (example fsm)
- `config/runtime.json` — runtime operational settings (for example session TTL)

### Machine definition shape

The machine supports:

- `name`
- `initial_state`
- `error_state`
- `states` map  
  Each state may define:
    - optional state-level `action`
    - `transitions` with:
        - `id`
        - `condition` missing condition defaults to 'true'
        - optional `action`
        - `target`

### Runtime config shape

`runtime.json` includes runtime settings such as:

- `session_ttl` (duration string, e.g. `"30m"`, `"1h"`)

---

## CLI usage

## Prerequisites

- Go **1.26+**

## Commands

```bash
./state-machine-engine [flags] # default behavior: send request 
./state-machine-engine serve [flags] # run server, required to have a server running to receive state-by-state requests
./state-machine-engine config [flags] # llm-powered workflow to assist in creating your state machine configuration
```

## Global flags (persistent)

- `--config` optional config file path (yaml/json/toml)
- `--machine` path to machine definition JSON (default: `config/machine.json`)
- `--runtime` path to runtime config JSON (default: `config/runtime.json`)
- `--socket` path to Unix socket
- `--input` JSON request envelope (used by root/default request flow)

## `serve` command

```bash
./state-machine-engine serve --machine config/machine.json --runtime config/runtime.json
```

Optional:
- `--socket` to override default Unix socket path

## Default request flow (root command)

```bash
./state-machine-engine --input '{"session_id":"s1","input":{"foo":"bar"}}'
```

This sends a request to the running service over the configured socket.

## `config` command

```bash
./state-machine-engine config --model gpt-4.1
```

Command flags:
- `--model` LLM model name

This starts an interactive session with a local ollama AI assistant to help you configure your state machine.

---

## Configuration precedence (Viper)

Values are resolved in this order:

1. Command-line flags
2. Environment variables
3. Config file (`--config`)
4. Default flag values

Environment variable prefix is `SME_`. Examples:

- `SME_MACHINE`
- `SME_RUNTIME`
- `SME_SOCKET`
- `SME_INPUT`
- `SME_MODEL`
- `SME_NON_INTERACTIVE`

---

## Extendability model

Extendability is preserved through runtime injection in `cmd/runtime.go`.

`Runtime` contract:

```go
type Runtime interface { 
    BuildService(machinePath, runtimePath string) *app.Service 
    RunRequest(socketPath, input string) error 
    RunConfigAssistant(machinePath, runtimePath, model string, nonInteractive bool) error 
}
```

The CLI only orchestrates commands and config.  
The runtime implementation decides adapter composition and behavior.

## Default runtime

The repository provides `NewDefaultRuntime()` as a batteries-included runtime wiring.

## Custom runtime (downstream app)

Downstream apps can fully control adapter composition by providing their own runtime:

```go
package main

type MyRuntime struct{}

func (r *MyRuntime) BuildService(machinePath, runtimePath string) *app.Service { 
    // Wire your own config provider, session store, validators, actions, conditions, observers. 
    panic("implement me") 
}

func (r *MyRuntime) RunRequest(socketPath, input string) error { 
    // Optionally customize transport/client behavior. 
    panic("implement me") 
}

func (r *MyRuntime) RunConfigAssistant(machinePath, runtimePath, model string, nonInteractive bool) error { 
    // Implement your own config-assistant flow. 
    panic("implement me") 
}

func main() { 
    cmd.Execute(&MyRuntime{}) 
}
```

This keeps the core engine reusable while allowing opinionated runtime composition per application.

# Samples

See sample implementations in the [sample](https://github.com/dannyroozen/state-machine-engine/tree/main/sample) directory.