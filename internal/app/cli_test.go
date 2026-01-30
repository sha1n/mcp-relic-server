package app

import (
	"testing"

	"github.com/spf13/pflag"
)

func TestRegisterFlags(t *testing.T) {
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	RegisterFlags(flags)

	// Verify all flags are registered
	expectedFlags := []string{
		"transport",
		"host",
		"port",
		"auth-type",
		"auth-basic-username",
		"auth-basic-password",
		"auth-api-keys",
	}

	for _, name := range expectedFlags {
		if flags.Lookup(name) == nil {
			t.Errorf("Expected flag %q to be registered", name)
		}
	}
}

func TestRegisterFlags_Shorthand(t *testing.T) {
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	RegisterFlags(flags)

	shorthandFlags := map[string]string{
		"transport":           "t",
		"host":                "H",
		"port":                "p",
		"auth-type":           "a",
		"auth-basic-username": "u",
		"auth-basic-password": "P",
		"auth-api-keys":       "k",
	}

	for name, shorthand := range shorthandFlags {
		flag := flags.Lookup(name)
		if flag == nil {
			t.Errorf("Flag %q not found", name)
			continue
		}
		if flag.Shorthand != shorthand {
			t.Errorf("Flag %q expected shorthand %q, got %q", name, shorthand, flag.Shorthand)
		}
	}
}

func TestRegisterFlags_SetValues(t *testing.T) {
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	RegisterFlags(flags)

	err := flags.Parse([]string{
		"--transport", "sse",
		"--host", "localhost",
		"--port", "9090",
		"--auth-type", "basic",
	})
	if err != nil {
		t.Fatalf("Failed to parse flags: %v", err)
	}

	transport, _ := flags.GetString("transport")
	if transport != "sse" {
		t.Errorf("Expected transport 'sse', got '%s'", transport)
	}

	host, _ := flags.GetString("host")
	if host != "localhost" {
		t.Errorf("Expected host 'localhost', got '%s'", host)
	}

	port, _ := flags.GetInt("port")
	if port != 9090 {
		t.Errorf("Expected port 9090, got %d", port)
	}

	authType, _ := flags.GetString("auth-type")
	if authType != "basic" {
		t.Errorf("Expected auth-type 'basic', got '%s'", authType)
	}
}
