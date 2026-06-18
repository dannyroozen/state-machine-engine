// Simple observer that logs the transition

package logging

import (
	"context"
	"encoding/json"
	"io"
	"sync"

	"state-machine-engine/internal/domain"
)

type Observer struct {
	w  io.Writer
	mu sync.Mutex
}

func NewObserver(w io.Writer) *Observer {
	return &Observer{w: w}
}

func (o *Observer) OnTransition(_ context.Context, event domain.TransitionEvent) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	b, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = o.w.Write(append(b, '\n'))
	return err
}
