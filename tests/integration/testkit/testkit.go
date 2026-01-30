package testkit

import (
	"fmt"
	"net"
	"testing"

	"github.com/sha1n/mcp-relic-server/internal/app"
	"github.com/spf13/pflag"
)

// Service represents a test service that can be started and stopped
type Service interface {
	Start() (map[string]any, error)
	Stop() error
	GetName() string
}

// TestEnvContext provides access to properties collected during environment startup
type TestEnvContext interface {
	GetProperties() map[string]any
	GetProperty(name string) (any, bool)
}

// TestEnv manages the lifecycle of test services
type TestEnv interface {
	Start() (map[string]any, error)
	Stop() error
	GetContext() TestEnvContext
}

type testEnvContextImpl struct {
	properties map[string]any
}

func (c *testEnvContextImpl) GetProperties() map[string]any {
	return c.properties
}

func (c *testEnvContextImpl) GetProperty(name string) (any, bool) {
	val, ok := c.properties[name]
	return val, ok
}

type testEnvImpl struct {
	services []Service
	context  *testEnvContextImpl
}

// NewTestEnv creates a new test environment with the given services
func NewTestEnv(services ...Service) TestEnv {
	return &testEnvImpl{
		services: services,
		context:  &testEnvContextImpl{properties: make(map[string]any)},
	}
}

func (e *testEnvImpl) Start() (map[string]any, error) {
	for _, s := range e.services {
		props, err := s.Start()
		if err != nil {
			return nil, err
		}
		for k, v := range props {
			e.context.properties[k] = v
		}
	}
	return e.context.properties, nil
}

func (e *testEnvImpl) Stop() error {
	var lastErr error
	// Stop in reverse order
	for i := len(e.services) - 1; i >= 0; i-- {
		if err := e.services[i].Stop(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

func (e *testEnvImpl) GetContext() TestEnvContext {
	return e.context
}

// GetFreePort returns a free port from the kernel
func GetFreePort() (int, error) {
	return getFreePortWithAddr("localhost:0")
}

// MustGetFreePort returns a free port or fails the test
func MustGetFreePort(t testing.TB) int {
	t.Helper()
	port, err := GetFreePort()
	if err != nil {
		t.Fatalf("Failed to get free port: %v", err)
	}
	return port
}

func getFreePortWithAddr(addrStr string) (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", addrStr)
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer func() { _ = l.Close() }()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// FlagOptions configures NewTestFlags
type FlagOptions struct {
	Port      int    // Uses free port if 0
	Transport string // Defaults to "sse"
	AuthType  string // Defaults to "none"
	Host      string // Defaults to "localhost"
}

// NewTestFlags creates a configured pflag.FlagSet for testing
func NewTestFlags(t testing.TB, opts *FlagOptions) *pflag.FlagSet {
	t.Helper()

	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	app.RegisterFlags(flags)

	port := 0
	transport := "sse"
	authType := "none"
	host := "localhost"

	if opts != nil {
		if opts.Port != 0 {
			port = opts.Port
		}
		if opts.Transport != "" {
			transport = opts.Transport
		}
		if opts.AuthType != "" {
			authType = opts.AuthType
		}
		if opts.Host != "" {
			host = opts.Host
		}
	}

	if port == 0 {
		port = MustGetFreePort(t)
	}

	_ = flags.Set("port", fmt.Sprintf("%d", port))
	_ = flags.Set("transport", transport)
	_ = flags.Set("auth-type", authType)
	_ = flags.Set("host", host)

	return flags
}
