package config

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestLog(t *testing.T) {
	// Just verify it doesn't panic
	s := &Settings{
		Transport: "sse",
		Host:      "localhost",
		Port:      8080,
		Auth: AuthSettings{
			Type: AuthTypeNone,
		},
	}
	Log(s) // Should not panic
}

func TestLogWithLogger_StdioTransport(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	s := &Settings{
		Transport: "stdio",
		Host:      "localhost",
		Port:      8080,
		Auth: AuthSettings{
			Type: AuthTypeNone,
		},
	}

	LogWithLogger(s, logger)

	output := buf.String()
	if !strings.Contains(output, "transport") {
		t.Error("Expected 'transport' in log output")
	}
	// stdio transport should not log host/port
	if strings.Contains(output, "host") {
		t.Error("Expected no 'host' in log output for stdio transport")
	}
}

func TestLogWithLogger_SSETransport(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	s := &Settings{
		Transport: "sse",
		Host:      "localhost",
		Port:      8080,
		Auth: AuthSettings{
			Type: AuthTypeNone,
		},
	}

	LogWithLogger(s, logger)

	output := buf.String()
	if !strings.Contains(output, "transport") {
		t.Error("Expected 'transport' in log output")
	}
	if !strings.Contains(output, "host") {
		t.Error("Expected 'host' in log output for SSE transport")
	}
	if !strings.Contains(output, "port") {
		t.Error("Expected 'port' in log output for SSE transport")
	}
}

func TestLogWithLogger_BasicAuth(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	s := &Settings{
		Transport: "stdio",
		Auth: AuthSettings{
			Type: AuthTypeBasic,
			Basic: BasicAuthSettings{
				Username: "admin",
				Password: "secret",
			},
		},
	}

	LogWithLogger(s, logger)

	output := buf.String()
	if !strings.Contains(output, "admin") {
		t.Error("Expected username in log output")
	}
	if !strings.Contains(output, "****") {
		t.Error("Expected masked password in log output")
	}
	if strings.Contains(output, "secret") {
		t.Error("Password should be masked, not shown in plain text")
	}
}

func TestLogWithLogger_APIKeyAuth(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	s := &Settings{
		Transport: "stdio",
		Auth: AuthSettings{
			Type:    AuthTypeAPIKey,
			APIKeys: []string{"key1", "key2", "key3"},
		},
	}

	LogWithLogger(s, logger)

	output := buf.String()
	if !strings.Contains(output, "count=3") {
		t.Errorf("Expected 'count=3' in log output, got: %s", output)
	}
}

func TestSettingsLogValue(t *testing.T) {
	s := Settings{
		Transport: "sse",
		Host:      "localhost",
		Port:      8080,
		Auth: AuthSettings{
			Type:    AuthTypeAPIKey,
			APIKeys: []string{"key1"},
		},
	}

	val := SettingsLogValue(s)
	if val.Kind() != slog.KindGroup {
		t.Errorf("Expected group kind, got %v", val.Kind())
	}
}

func TestAuthSettingsLogValue(t *testing.T) {
	s := AuthSettings{
		Type:    AuthTypeAPIKey,
		APIKeys: []string{"key1", "key2"},
		Basic: BasicAuthSettings{
			Username: "user",
			Password: "pass",
		},
	}

	val := AuthSettingsLogValue(s)
	if val.Kind() != slog.KindGroup {
		t.Errorf("Expected group kind, got %v", val.Kind())
	}
}

func TestBasicAuthSettingsLogValue(t *testing.T) {
	s := BasicAuthSettings{
		Username: "admin",
		Password: "secret",
	}

	val := BasicAuthSettingsLogValue(s)
	if val.Kind() != slog.KindGroup {
		t.Errorf("Expected group kind, got %v", val.Kind())
	}
}
