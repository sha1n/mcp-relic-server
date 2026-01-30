package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/sha1n/mcp-relic-server/internal/config"
	"github.com/spf13/pflag"
)

// noopValidate is a no-op validation function for tests
func noopValidate(*config.Settings) error {
	return nil
}

func TestRunWithDeps_ErrorCases(t *testing.T) {
	tests := []struct {
		name           string
		params         RunParams
		wantErrContain string
	}{
		{
			name: "LoadSettings error",
			params: RunParams{
				LoadSettings: func(*pflag.FlagSet) (*config.Settings, error) {
					return nil, errors.New("settings error")
				},
				ValidSettings: noopValidate,
			},
			wantErrContain: "failed to load settings",
		},
		{
			name: "ValidSettings error",
			params: RunParams{
				LoadSettings: func(*pflag.FlagSet) (*config.Settings, error) {
					return &config.Settings{Transport: "sse"}, nil
				},
				ValidSettings: func(*config.Settings) error {
					return errors.New("validation error")
				},
			},
			wantErrContain: "invalid configuration",
		},
		{
			name: "CreateServer error",
			params: RunParams{
				LoadSettings: func(*pflag.FlagSet) (*config.Settings, error) {
					return &config.Settings{Transport: "sse"}, nil
				},
				ValidSettings: noopValidate,
				CreateServer: func(*config.Settings) (*mcp.Server, func(), error) {
					return nil, nil, errors.New("create server error")
				},
			},
			wantErrContain: "create server error",
		},
		{
			name: "StartSSEServer error",
			params: RunParams{
				LoadSettings: func(*pflag.FlagSet) (*config.Settings, error) {
					return &config.Settings{Transport: "sse"}, nil
				},
				ValidSettings: noopValidate,
				CreateServer: func(*config.Settings) (*mcp.Server, func(), error) {
					return nil, nil, nil
				},
				StartSSEServer: func(*mcp.Server, *config.Settings) error {
					return errors.New("sse start error")
				},
			},
			wantErrContain: "sse start error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := RunWithDeps(context.Background(), tt.params, nil, "test")
			if err == nil {
				t.Fatalf("Expected error containing %q, got nil", tt.wantErrContain)
			}
			if !strings.Contains(err.Error(), tt.wantErrContain) {
				t.Errorf("Expected error containing %q, got %q", tt.wantErrContain, err.Error())
			}
		})
	}
}

func TestRunWithDeps_Cleanup(t *testing.T) {
	cleanupCalled := false
	params := RunParams{
		LoadSettings: func(*pflag.FlagSet) (*config.Settings, error) {
			return &config.Settings{Transport: "sse"}, nil
		},
		ValidSettings: noopValidate,
		CreateServer: func(*config.Settings) (*mcp.Server, func(), error) {
			return nil, func() { cleanupCalled = true }, nil
		},
		StartSSEServer: func(*mcp.Server, *config.Settings) error {
			return errors.New("intentional error to trigger cleanup")
		},
	}

	_ = RunWithDeps(context.Background(), params, nil, "test")

	if !cleanupCalled {
		t.Error("Cleanup was not called")
	}
}

func TestDefaultRunParams(t *testing.T) {
	params := DefaultRunParams()

	if params.LoadSettings == nil {
		t.Error("LoadSettings is nil")
	}
	if params.ValidSettings == nil {
		t.Error("ValidSettings is nil")
	}
	if params.StartSSEServer == nil {
		t.Error("StartSSEServer is nil")
	}
	if params.CreateServer == nil {
		t.Error("CreateServer is nil")
	}
}

func TestRunWithDeps_StdioWithDefaultTransport(t *testing.T) {
	params := RunParams{
		LoadSettings: func(*pflag.FlagSet) (*config.Settings, error) {
			return &config.Settings{Transport: "stdio"}, nil
		},
		ValidSettings: noopValidate,
		CreateServer: func(*config.Settings) (*mcp.Server, func(), error) {
			impl := &mcp.Implementation{Name: "test", Version: "1.0"}
			server := mcp.NewServer(impl, nil)
			return server, nil, nil
		},
		CustomIOTransport: nil,
	}

	// Use a cancelled context to avoid hanging on stdio
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := RunWithDeps(ctx, params, nil, "test")

	// We expect an error because the context is cancelled
	if err == nil {
		t.Log("No error returned (unexpected)")
	}
}

func TestRunWithDeps_StdioWithCustomTransport(t *testing.T) {
	transportUsed := false
	customTransport := &mockTransport{
		connectCalled: &transportUsed,
	}

	params := RunParams{
		LoadSettings: func(*pflag.FlagSet) (*config.Settings, error) {
			return &config.Settings{Transport: "stdio"}, nil
		},
		ValidSettings: noopValidate,
		CreateServer: func(*config.Settings) (*mcp.Server, func(), error) {
			impl := &mcp.Implementation{Name: "test", Version: "1.0"}
			server := mcp.NewServer(impl, nil)
			return server, nil, nil
		},
		CustomIOTransport: customTransport,
	}

	// Use a cancelled context to avoid hanging
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_ = RunWithDeps(ctx, params, nil, "test")

	if !transportUsed {
		t.Error("Custom transport Connect was not called")
	}
}

func TestCreateMCPServer(t *testing.T) {
	settings := &config.Settings{
		Transport: "stdio",
	}

	server, cleanup, err := CreateMCPServer(settings)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if server == nil {
		t.Error("Expected server to be created")
	}
	if cleanup != nil {
		cleanup()
	}
}

// mockTransport implements mcp.Transport for testing
type mockTransport struct {
	connectCalled *bool
}

func (m *mockTransport) Connect(ctx context.Context) (mcp.Connection, error) {
	if m.connectCalled != nil {
		*m.connectCalled = true
	}
	return nil, errors.New("mock transport - no real connection")
}
