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
	mux.HandleFunc("GET /stats", func(w http.ResponseWriter, r *http.Request) {
		handleStats(service, w, r)
	})
	mux.HandleFunc("DELETE /sessions/expired", func(w http.ResponseWriter, r *http.Request) {
		handleDeleteExpired(service, w, r)
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

func handleStats(service *app.Service, w http.ResponseWriter, r *http.Request) {
	logger.Debug("entered handleStats")

	stats, err := service.Stats(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "stats_failed", err.Error())
		return
	}

	responseBody, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		logger.Error(fmt.Sprintf("failed to prepare stats response: %v", err))
		writeError(w, http.StatusInternalServerError, "stats_failed", err.Error())
		return
	}

	logger.Debug(fmt.Sprintf("handleStats response body: %s", string(responseBody)))

	w.Header().Set("Content-Type", "application/json")
	if _, err = w.Write(append(responseBody, '\n')); err != nil {
		logger.Error(fmt.Sprintf("failed to write stats response: %v", err))
	}
}

// handleDeleteExpired removes all expired sessions and returns the count deleted.
func handleDeleteExpired(service *app.Service, w http.ResponseWriter, r *http.Request) {
	logger.Debug("entered handleDeleteExpired")

	deleted, err := service.DeleteExpired(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "delete_expired_failed", err.Error())
		return
	}

	responseBody, err := json.MarshalIndent(map[string]int{"deleted": deleted}, "", "  ")
	if err != nil {
		logger.Error(fmt.Sprintf("failed to prepare delete_expired response: %v", err))
		writeError(w, http.StatusInternalServerError, "delete_expired_failed", err.Error())
		return
	}

	logger.Debug(fmt.Sprintf("handleDeleteExpired response body: %s", string(responseBody)))

	w.Header().Set("Content-Type", "application/json")
	if _, err = w.Write(append(responseBody, '\n')); err != nil {
		logger.Error(fmt.Sprintf("failed to write delete_expired response: %v", err))
	}
}

func handleRequest(service *app.Service, w http.ResponseWriter, r *http.Request) {
	logger.Debug("entered handleRequest")

	// limit size of request, especially useful in our implementation so that we can switch to a tcp REST sever in the future without concern
	body, err := io.ReadAll(io.LimitReader(r.Body, domain.MaxRequestSizeBytes+1))
	if err != nil {
		writeError(w, http.StatusBadRequest, "read_failed", err.Error())
		return
	}

	logger.Debug(fmt.Sprintf("handleRequest request body: %s", string(body)))

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

	transportResp := makeTransportResponse(resp)
	responseBody, err := json.MarshalIndent(transportResp, "", "  ")
	if err != nil {
		logger.Error(fmt.Sprintf("failed to prepare response: %v", err))
		writeError(w, http.StatusInternalServerError, "process_failed", err.Error())
		return
	}

	logger.Debug(fmt.Sprintf("handleRequest response body: %s", string(responseBody)))

	w.Header().Set("Content-Type", "application/json")
	if _, err = w.Write(append(responseBody, '\n')); err != nil {
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
func RunClient(socketPath, input, endpoint string) error {
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "unix", socketPath)
			},
		},
		Timeout: 30 * time.Second,
	}

	method := http.MethodPost
	if endpoint == "" {
		endpoint = "requests"
	} else {
		endpoint = strings.TrimPrefix(endpoint, "/")
		switch endpoint {
		case "stats":
			method = http.MethodGet
		case "sessions/expired":
			method = http.MethodDelete
		}
	}

	var body io.Reader
	if method == http.MethodPost {
		body = bytes.NewReader([]byte(input))
	}

	req, err := http.NewRequest(method, "http://state-machine/"+endpoint, body)
	if err != nil {
		return fmt.Errorf("failed to build request: %w", err)
	}
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := client.Do(req)
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
