package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"autotest/internal/spec"

	"github.com/google/uuid"
)

const maxSpecBytes = 20 << 20

// APIClient calls autotest REST API using an API Key (Bearer at-...).
type APIClient struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

func NewAPIClient(cfg Config) *APIClient {
	return &APIClient{
		baseURL: cfg.APIBaseURL,
		apiKey:  cfg.APIKey,
		http: &http.Client{
			Timeout: 2 * time.Minute,
		},
	}
}

// ImportSpec posts OpenAPI/Swagger document bytes to the specs import endpoint.
func (c *APIClient) ImportSpec(ctx context.Context, projectID, serviceID uuid.UUID, content []byte, contentType string) (*spec.ImportSummary, error) {
	if len(content) == 0 {
		return nil, fmt.Errorf("spec content is empty")
	}
	if len(content) > maxSpecBytes {
		return nil, fmt.Errorf("spec content exceeds %d bytes limit", maxSpecBytes)
	}
	if contentType == "" {
		contentType = detectContentType(content)
	}

	url := fmt.Sprintf("%s/projects/%s/services/%s/specs/import", c.baseURL, projectID, serviceID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(content))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", contentType)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request import: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusCreated {
		return nil, parseAPIError(resp.StatusCode, body)
	}

	var summary spec.ImportSummary
	if err := json.Unmarshal(body, &summary); err != nil {
		return nil, fmt.Errorf("decode import summary: %w", err)
	}
	return &summary, nil
}

func parseAPIError(status int, body []byte) error {
	var er struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(body, &er) == nil && er.Error != "" {
		return fmt.Errorf("import failed (%d): %s", status, er.Error)
	}
	msg := strings.TrimSpace(string(body))
	if msg == "" {
		msg = http.StatusText(status)
	}
	return fmt.Errorf("import failed (%d): %s", status, msg)
}

func detectContentType(data []byte) string {
	trim := bytes.TrimSpace(data)
	if len(trim) > 0 && trim[0] == '{' {
		return "application/json"
	}
	return "application/yaml"
}
