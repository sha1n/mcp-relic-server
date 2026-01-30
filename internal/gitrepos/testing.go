package gitrepos

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// MockExecutor records commands and returns configured responses.
// This is exported for use in integration tests.
type MockExecutor struct {
	commands []MockCommand
	calls    []ExecutorCall
}

// MockCommand defines a mock response for a command prefix.
type MockCommand struct {
	NamePrefix string
	Output     []byte
	Err        error
}

// ExecutorCall records a command invocation.
type ExecutorCall struct {
	Dir  string
	Name string
	Args []string
}

// NewMockExecutor creates a new mock executor.
func NewMockExecutor() *MockExecutor {
	return &MockExecutor{
		commands: make([]MockCommand, 0),
		calls:    make([]ExecutorCall, 0),
	}
}

// AddResponse adds a mock response for commands matching the given prefix.
func (m *MockExecutor) AddResponse(namePrefix string, output []byte, err error) {
	m.commands = append(m.commands, MockCommand{
		NamePrefix: namePrefix,
		Output:     output,
		Err:        err,
	})
}

// Run executes a command and returns the configured mock response.
func (m *MockExecutor) Run(_ context.Context, dir string, name string, args ...string) ([]byte, error) {
	call := ExecutorCall{Dir: dir, Name: name, Args: args}
	m.calls = append(m.calls, call)

	// Build full command string for matching
	fullCmd := name + " " + strings.Join(args, " ")

	// Find matching response
	for i, cmd := range m.commands {
		if strings.HasPrefix(fullCmd, cmd.NamePrefix) {
			// Remove used response
			m.commands = append(m.commands[:i], m.commands[i+1:]...)
			return cmd.Output, cmd.Err
		}
	}

	return nil, errors.New("no mock response configured for: " + fullCmd)
}

// GetCalls returns all recorded command calls.
func (m *MockExecutor) GetCalls() []ExecutorCall {
	return m.calls
}

// MustGetLastCall returns the last recorded call, fails the test if no calls were made.
func (m *MockExecutor) MustGetLastCall(t *testing.T) ExecutorCall {
	t.Helper()
	if len(m.calls) == 0 {
		t.Fatal("Expected at least one command call")
	}
	return m.calls[len(m.calls)-1]
}
