package testkit

import (
	"errors"
	"testing"
)

// Mock service for testing
type mockService struct {
	name       string
	startProps map[string]any
	startErr   error
	stopErr    error
	started    bool
	stopped    bool
	onStop     func() // Optional callback when Stop is called
}

func (m *mockService) Start() (map[string]any, error) {
	m.started = true
	return m.startProps, m.startErr
}

func (m *mockService) Stop() error {
	m.stopped = true
	if m.onStop != nil {
		m.onStop()
	}
	return m.stopErr
}

func (m *mockService) GetName() string {
	return m.name
}

func TestNewTestEnv(t *testing.T) {
	svc := &mockService{name: "test-service"}
	env := NewTestEnv(svc)

	if env == nil {
		t.Fatal("Expected non-nil TestEnv")
	}

	ctx := env.GetContext()
	if ctx == nil {
		t.Fatal("Expected non-nil context")
	}

	props := ctx.GetProperties()
	if props == nil {
		t.Fatal("Expected non-nil properties map")
	}
	if len(props) != 0 {
		t.Errorf("Expected empty properties, got %d", len(props))
	}
}

func TestTestEnvStart(t *testing.T) {
	t.Run("single service success", func(t *testing.T) {
		svc := &mockService{
			name:       "svc1",
			startProps: map[string]any{"port": 8080},
		}
		env := NewTestEnv(svc)

		props, err := env.Start()
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !svc.started {
			t.Error("Service should have been started")
		}
		if props["port"] != 8080 {
			t.Errorf("Expected port 8080, got %v", props["port"])
		}
	})

	t.Run("multiple services merge properties", func(t *testing.T) {
		svc1 := &mockService{
			name:       "svc1",
			startProps: map[string]any{"key1": "value1"},
		}
		svc2 := &mockService{
			name:       "svc2",
			startProps: map[string]any{"key2": "value2"},
		}
		env := NewTestEnv(svc1, svc2)

		props, err := env.Start()
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if props["key1"] != "value1" {
			t.Errorf("Expected key1=value1, got %v", props["key1"])
		}
		if props["key2"] != "value2" {
			t.Errorf("Expected key2=value2, got %v", props["key2"])
		}
	})

	t.Run("start error", func(t *testing.T) {
		svc := &mockService{
			name:     "failing-svc",
			startErr: errors.New("start failed"),
		}
		env := NewTestEnv(svc)

		_, err := env.Start()
		if err == nil {
			t.Fatal("Expected error")
		}
		if err.Error() != "start failed" {
			t.Errorf("Expected 'start failed', got %v", err)
		}
	})
}

func TestTestEnvStop(t *testing.T) {
	t.Run("stops in reverse order", func(t *testing.T) {
		stopOrder := []string{}
		svc1 := &mockService{
			name: "svc1",
			onStop: func() {
				stopOrder = append(stopOrder, "svc1")
			},
		}
		svc2 := &mockService{
			name: "svc2",
			onStop: func() {
				stopOrder = append(stopOrder, "svc2")
			},
		}

		env := NewTestEnv(svc1, svc2)
		_, _ = env.Start()
		_ = env.Stop()

		if len(stopOrder) != 2 {
			t.Fatalf("Expected 2 stops, got %d", len(stopOrder))
		}
		if stopOrder[0] != "svc2" || stopOrder[1] != "svc1" {
			t.Errorf("Expected reverse order [svc2, svc1], got %v", stopOrder)
		}
	})

	t.Run("returns last error", func(t *testing.T) {
		svc1 := &mockService{name: "svc1", stopErr: errors.New("error1")}
		svc2 := &mockService{name: "svc2", stopErr: errors.New("error2")}
		env := NewTestEnv(svc1, svc2)

		err := env.Stop()
		// svc2 stops first (reverse order), then svc1 - so svc1's error is "last"
		if err == nil || err.Error() != "error1" {
			t.Errorf("Expected 'error1', got %v", err)
		}
	})
}

func TestTestEnvContext(t *testing.T) {
	svc := &mockService{
		name:       "svc",
		startProps: map[string]any{"key": "value"},
	}
	env := NewTestEnv(svc)
	_, _ = env.Start()

	ctx := env.GetContext()

	t.Run("GetProperty found", func(t *testing.T) {
		val, ok := ctx.GetProperty("key")
		if !ok {
			t.Error("Expected property to be found")
		}
		if val != "value" {
			t.Errorf("Expected 'value', got %v", val)
		}
	})

	t.Run("GetProperty not found", func(t *testing.T) {
		_, ok := ctx.GetProperty("nonexistent")
		if ok {
			t.Error("Expected property not to be found")
		}
	})
}

func TestGetFreePort(t *testing.T) {
	port, err := GetFreePort()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if port <= 0 {
		t.Errorf("Expected positive port, got %d", port)
	}

	// Verify we get different ports on subsequent calls
	port2, err := GetFreePort()
	if err != nil {
		t.Fatalf("Unexpected error on second call: %v", err)
	}
	// Note: ports might be the same if reused, but typically different
	if port2 <= 0 {
		t.Errorf("Expected positive port, got %d", port2)
	}
}

func TestMustGetFreePort(t *testing.T) {
	port := MustGetFreePort(t)
	if port <= 0 {
		t.Errorf("Expected positive port, got %d", port)
	}
}

func TestGetFreePortWithAddr_InvalidAddr(t *testing.T) {
	_, err := getFreePortWithAddr("invalid:address:format")
	if err == nil {
		t.Error("Expected error for invalid address")
	}
}

func TestNewTestFlags(t *testing.T) {
	t.Run("default options", func(t *testing.T) {
		flags := NewTestFlags(t, nil)

		transport, _ := flags.GetString("transport")
		if transport != "sse" {
			t.Errorf("Expected transport 'sse', got %s", transport)
		}

		authType, _ := flags.GetString("auth-type")
		if authType != "none" {
			t.Errorf("Expected auth-type 'none', got %s", authType)
		}

		host, _ := flags.GetString("host")
		if host != "localhost" {
			t.Errorf("Expected host 'localhost', got %s", host)
		}

		port, _ := flags.GetInt("port")
		if port <= 0 {
			t.Errorf("Expected positive port, got %d", port)
		}
	})

	t.Run("custom options", func(t *testing.T) {
		flags := NewTestFlags(t, &FlagOptions{
			Port:      9999,
			Transport: "stdio",
			AuthType:  "basic",
			Host:      "127.0.0.1",
		})

		port, _ := flags.GetInt("port")
		if port != 9999 {
			t.Errorf("Expected port 9999, got %d", port)
		}

		transport, _ := flags.GetString("transport")
		if transport != "stdio" {
			t.Errorf("Expected transport 'stdio', got %s", transport)
		}

		authType, _ := flags.GetString("auth-type")
		if authType != "basic" {
			t.Errorf("Expected auth-type 'basic', got %s", authType)
		}

		host, _ := flags.GetString("host")
		if host != "127.0.0.1" {
			t.Errorf("Expected host '127.0.0.1', got %s", host)
		}
	})

	t.Run("auto-assign port when zero", func(t *testing.T) {
		flags := NewTestFlags(t, &FlagOptions{Port: 0})

		port, _ := flags.GetInt("port")
		if port <= 0 {
			t.Errorf("Expected auto-assigned positive port, got %d", port)
		}
	})
}
