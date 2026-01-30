package config

import (
	"context"
	"log/slog"
)

// Log logs the resolved settings in a granular way, skipping irrelevant ones
func Log(s *Settings) {
	LogWithLogger(s, slog.Default())
}

// LogWithLogger logs the resolved settings using the provided logger
func LogWithLogger(s *Settings, logger *slog.Logger) {
	ctx := context.Background()
	logger.InfoContext(ctx, "Config: transport", "value", s.Transport)
	if s.Transport == "sse" {
		logger.InfoContext(ctx, "Config: host", "value", s.Host)
		logger.InfoContext(ctx, "Config: port", "value", s.Port)
	}

	logger.InfoContext(ctx, "Config: auth.type", "value", s.Auth.Type)
	switch s.Auth.Type {
	case AuthTypeBasic:
		logger.InfoContext(ctx, "Config: auth.basic.username", "value", s.Auth.Basic.Username)
		logger.InfoContext(ctx, "Config: auth.basic.password", "value", "****")
	case AuthTypeAPIKey:
		logger.InfoContext(ctx, "Config: auth.api_keys", "count", len(s.Auth.APIKeys))
	}
}

// AuthSettingsLogValue returns a slog.Value for AuthSettings with masked data
func AuthSettingsLogValue(s AuthSettings) slog.Value {
	keys := make([]string, len(s.APIKeys))
	for i := range s.APIKeys {
		keys[i] = "****"
	}
	return slog.GroupValue(
		slog.String("type", s.Type),
		slog.Any("basic", BasicAuthSettingsLogValue(s.Basic)),
		slog.Any("api_keys", keys),
	)
}

// BasicAuthSettingsLogValue returns a slog.Value for BasicAuthSettings with masked data
func BasicAuthSettingsLogValue(s BasicAuthSettings) slog.Value {
	return slog.GroupValue(
		slog.String("username", s.Username),
		slog.String("password", "****"),
	)
}

// SettingsLogValue returns a slog.Value for Settings with masked data
func SettingsLogValue(s Settings) slog.Value {
	return slog.GroupValue(
		slog.String("transport", s.Transport),
		slog.String("host", s.Host),
		slog.Int("port", s.Port),
		slog.Any("auth", AuthSettingsLogValue(s.Auth)),
	)
}
