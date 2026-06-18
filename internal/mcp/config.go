package mcp

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

const (
	envAPIBaseURL     = "AUTOTEST_API_BASE_URL"
	envAPIKey         = "AUTOTEST_API_KEY"
	envProjectID      = "AUTOTEST_PROJECT_ID"
	envServiceID      = "AUTOTEST_SERVICE_ID"
	envEnvironmentID  = "AUTOTEST_ENVIRONMENT_ID"

	defaultAPIBaseURL = "http://localhost:8080/api/v1"
)

// Config holds connection settings for the MCP server to call autotest HTTP API.
type Config struct {
	APIBaseURL    string
	APIKey        string
	ProjectID     string
	ServiceID     string
	EnvironmentID string
}

// LoadConfigFromEnv reads MCP settings from environment variables.
// AUTOTEST_API_KEY is required; project/service/environment IDs are optional defaults for tools.
func LoadConfigFromEnv() (Config, error) {
	cfg := Config{
		APIBaseURL:    strings.TrimRight(strings.TrimSpace(os.Getenv(envAPIBaseURL)), "/"),
		APIKey:        strings.TrimSpace(os.Getenv(envAPIKey)),
		ProjectID:     strings.TrimSpace(os.Getenv(envProjectID)),
		ServiceID:     strings.TrimSpace(os.Getenv(envServiceID)),
		EnvironmentID: strings.TrimSpace(os.Getenv(envEnvironmentID)),
	}
	if cfg.APIBaseURL == "" {
		cfg.APIBaseURL = defaultAPIBaseURL
	}
	if cfg.APIKey == "" {
		return Config{}, errors.New(envAPIKey + " is required (create an API Key with scopes such as specs:import, cases:read/write, scenarios:read/write, runs:read/execute)")
	}
	if !strings.HasPrefix(cfg.APIKey, "at-") {
		return Config{}, fmt.Errorf("%s must start with at-", envAPIKey)
	}
	return cfg, nil
}
