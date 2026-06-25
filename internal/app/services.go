package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"state-machine-engine/internal/domain"
	"state-machine-engine/internal/engine"
)

var (
	ErrInvalidRequest    = errors.New("invalid request")
	ErrRequestTooLarge   = errors.New("request exceeds 1 MB")
	ErrMissingSessionID  = errors.New("missing session_id")
	ErrSessionExpired    = errors.New("session expired")
	ErrDependencyMissing = errors.New("required dependency is not configured")
)

type Service struct {
	machine   domain.ConfigProvider
	config    domain.ConfigProvider
	sessions  domain.SessionStore
	validator domain.Validator
	engine    *engine.Engine
}

// NewService sets up the service to process requests and run the state machine engine
// machine and config are both ConfigProviders, but as separate arguments we allow for each to be configured differently
// (e.g. runtime config loaded via json, while state machine comes from the scxml standard)
func NewService(
	machine domain.ConfigProvider,
	config domain.ConfigProvider,
	sessions domain.SessionStore,
	validator domain.Validator,
	conditions domain.ConditionEvaluator,
	actions domain.ActionExecutor,
	observers []domain.Observer,
) *Service {
	return &Service{
		machine:   machine,
		config:    config,
		sessions:  sessions,
		validator: validator,
		engine:    engine.NewEngine(conditions, actions, observers),
	}
}

func (s *Service) ProcessRequest(
	ctx context.Context,
	req domain.RequestEnvelope,
	rawRequest []byte,
) (domain.ResponseEnvelope, error) {
	if len(rawRequest) > domain.MaxRequestSizeBytes { // limit size of incoming requests
		return domain.ResponseEnvelope{}, ErrRequestTooLarge
	}
	if req.SessionID == "" { // supports multiple concurrent sessions, so session ID required
		return domain.ResponseEnvelope{}, ErrMissingSessionID
	}
	if len(req.Input) > 0 && !json.Valid(req.Input) {
		return domain.ResponseEnvelope{}, fmt.Errorf("%w: input must be valid JSON", ErrInvalidRequest)
	}

	// Step 1 returns a clear dependency error until adapters are wired.
	if s.machine == nil || s.config == nil || s.sessions == nil || s.validator == nil || s.engine == nil {
		return domain.ResponseEnvelope{}, ErrDependencyMissing
	}

	// TODO: loading the state machine here would allow us to change the state machine during runtime, but loading it on creation saves time/processing during request
	machine, err := s.machine.LoadStateMachine(ctx)
	if err != nil {
		return domain.ResponseEnvelope{}, err
	}
	if err = s.validator.Validate(machine, s.engine); err != nil {
		return domain.ResponseEnvelope{}, err
	}

	rtCfg, err := s.config.LoadRuntimeConfig(ctx)
	if err != nil {
		return domain.ResponseEnvelope{}, err
	}

	session, err := s.sessions.Get(ctx, req.SessionID)
	if err != nil {
		return domain.ResponseEnvelope{}, err
	}
	if session == nil { // session ID not found, so assume new session
		session = &domain.Session{
			ID:    req.SessionID,
			State: machine.Initial,
		}
	}

	now := time.Now()
	if !session.ExpiresAt.IsZero() && now.After(session.ExpiresAt) {
		// This will store all sessions forever and simply reject future uses of the same session ID
		// TODO: Consider removing the session, perhaps in an asynchronous routine that evaluates all sessions, to remove stale sessions, perhaps after a certain amount of time
		// Though that could be unnecessary if the underlying session manager handles stale sessions itself, like DynamoDB.
		return domain.ResponseEnvelope{}, ErrSessionExpired
	}

	output, err := s.engine.Step(ctx, machine, req, session)
	if err != nil {
		if upsertError := s.sessions.Upsert(ctx, session); upsertError != nil {
			return domain.ResponseEnvelope{}, upsertError
		}
		return domain.ResponseEnvelope{
			SessionID: req.SessionID,
			State:     session.State,
			Error: &domain.ErrorBlock{
				Code:    "transition_or_action_failed",
				Message: err.Error(),
			},
		}, nil
	}

	// length of session increased at each step; for cleanup purposes in case the session is abandoned
	session.ExpiresAt = now.Add(rtCfg.Session.TTL)
	if err = s.sessions.Upsert(ctx, session); err != nil {
		return domain.ResponseEnvelope{}, err
	}

	return domain.ResponseEnvelope{
		SessionID: req.SessionID,
		State:     session.State,
		Output:    output,
	}, nil
}
