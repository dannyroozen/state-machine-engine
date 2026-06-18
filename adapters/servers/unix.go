package servers

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"state-machine-engine/internal/app"
	"state-machine-engine/internal/domain"
	"state-machine-engine/internal/logging"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"
)

var logger = logging.NewLogger("servers")

// StartAndListen starts the unix socket and listens for incoming requests
func StartAndListen(socketPath string, service *app.Service) error {
	// Refuse to start if another server is already listening; clean up a stale socket otherwise.
	if conn, err := net.DialTimeout("unix", socketPath, time.Second); err == nil {
		_ = conn.Close()
		return fmt.Errorf("server already running on %s", socketPath)
	}
	if err := os.Remove(socketPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to remove stale socket: %w", err)
	}

	// Start server listener
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", socketPath, err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /requests", func(w http.ResponseWriter, r *http.Request) {
		handleRequest(service, w, r) // one handler to rule them all
	})

	server := &http.Server{Handler: mux}

	// unix sockets should be cleaned up on termination
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// start up server and listen for errors
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Serve(listener)
	}()

	logger.Info(fmt.Sprintf("listening on %s", socketPath))

	select {
	case <-ctx.Done():
		// perform clean shutdown
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err = server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown failed: %w", err)
		}
		return nil
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

// handleRequest will route and process all incoming requests
func handleRequest(service *app.Service, w http.ResponseWriter, r *http.Request) {
	// limit size of request, especially useful in our implementation so that we can switch to a tcp REST sever in the future without concern
	body, err := io.ReadAll(io.LimitReader(r.Body, domain.MaxRequestSizeBytes+1))
	if err != nil {
		writeError(w, http.StatusBadRequest, "read_failed", err.Error())
		return
	}

	var req domain.RequestEnvelope
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	resp, err := service.ProcessRequest(r.Context(), req, body)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "process_failed", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err = enc.Encode(makeTransportResponse(resp)); err != nil {
		logger.Error(fmt.Sprintf("failed to write response: %v", err))
	}
}

type transportResponseEnvelope struct {
	SessionID string             `json:"session_id"`
	State     string             `json:"state"`
	Output    any                `json:"output,omitempty"`
	Error     *domain.ErrorBlock `json:"error,omitempty"`
}

func makeTransportResponse(resp domain.ResponseEnvelope) transportResponseEnvelope {
	return transportResponseEnvelope{
		SessionID: resp.SessionID,
		State:     resp.State,
		Output:    decodeFlexibleOutput(resp.Output),
		Error:     resp.Error,
	}
}

func decodeFlexibleOutput(out []byte) any {
	if len(out) == 0 {
		return nil
	}

	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return nil
	}

	raw := []byte(trimmed)
	if json.Valid(raw) {
		var parsed any
		if err := json.Unmarshal(raw, &parsed); err == nil {
			return parsed
		}
		return json.RawMessage(raw)
	}

	if utf8.Valid(raw) {
		return map[string]any{
			"format": "text",
			"data":   string(raw),
		}
	}

	return map[string]any{
		"format": "base64",
		"data":   base64.StdEncoding.EncodeToString(raw),
	}
}

// writeError outputs the error to the REST connection
func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(domain.ResponseEnvelope{
		Error: &domain.ErrorBlock{Code: code, Message: message},
	})
}

// RunClient handles non-server executions, sending a new request to an assumed running server
func RunClient(socketPath, input string) error {
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "unix", socketPath)
			},
		},
		Timeout: 30 * time.Second,
	}

	// Host is a placeholder; routing happens over the unix socket.
	resp, err := client.Post("http://state-machine/requests", "application/json", bytes.NewReader([]byte(input)))
	if err != nil {
		return fmt.Errorf("is the server running? start it with -serve. dial error: %w", err)
	}
	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			logger.Warn(fmt.Sprintf("error closing connection: %v", err))
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		_, err = fmt.Fprintln(os.Stderr, string(body))
		if err != nil {
			return fmt.Errorf("server returned status %s: %w", resp.Status, err)
		}

		return fmt.Errorf("server returned status %s", resp.Status)
	}

	if _, err = io.Copy(os.Stdout, resp.Body); err != nil {
		return fmt.Errorf("failed to write response: %w", err)
	}
	return nil
}
