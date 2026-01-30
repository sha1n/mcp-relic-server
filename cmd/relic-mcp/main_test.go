package main

import (
	"strings"
	"testing"
)

func TestExecute_Version(t *testing.T) {
	err := Execute("1.0.0", "abc123", "relic-mcp", []string{"--version"})
	if err != nil {
		t.Errorf("Expected no error for --version, got: %v", err)
	}
}

func TestExecute_Help(t *testing.T) {
	err := Execute("1.0.0", "abc123", "relic-mcp", []string{"--help"})
	if err != nil {
		t.Errorf("Expected no error for --help, got: %v", err)
	}
}

func TestExecute_InvalidFlag(t *testing.T) {
	err := Execute("1.0.0", "abc123", "relic-mcp", []string{"--invalid-flag"})
	if err == nil {
		t.Error("Expected error for invalid flag")
	}
}

func TestExecute_InvalidTransport(t *testing.T) {
	err := Execute("1.0.0", "abc123", "relic-mcp", []string{"--transport", "invalid"})
	if err == nil {
		t.Error("Expected error for invalid transport")
	}
	if !strings.Contains(err.Error(), "transport") {
		t.Errorf("Expected error about transport, got: %v", err)
	}
}

func TestRunMain_Success(t *testing.T) {
	exitCode := -1
	mockExit := func(code int) {
		exitCode = code
	}

	// --help should succeed
	runMain([]string{"relic-mcp", "--help"}, mockExit)

	if exitCode != -1 {
		t.Errorf("Expected no exit call for --help, got exit code: %d", exitCode)
	}
}

func TestRunMain_Failure(t *testing.T) {
	exitCode := -1
	mockExit := func(code int) {
		exitCode = code
	}

	runMain([]string{"relic-mcp", "--invalid"}, mockExit)

	if exitCode != 1 {
		t.Errorf("Expected exit code 1 for invalid flag, got: %d", exitCode)
	}
}
