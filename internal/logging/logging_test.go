package logging

import "testing"

func TestGetEnvDefault(t *testing.T) {
	t.Parallel()

	if got := getEnv(); got == "" {
		t.Fatal("expected non-empty env")
	}
}

func TestGetAppNameDefault(t *testing.T) {
	t.Parallel()

	if got := getAppName(); got == "" {
		t.Fatal("expected non-empty app name")
	}
}

func TestNewLogger_NotNil(t *testing.T) {
	t.Parallel()

	l := NewLogger("test")
	if l == nil {
		t.Fatal("expected logger instance")
	}
}
