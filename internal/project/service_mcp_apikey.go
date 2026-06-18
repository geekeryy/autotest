package project

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"autotest/internal/apikey"

	"github.com/google/uuid"
)

// McpAPIKeyResult holds a freshly issued token or an existing key mask.
type McpAPIKeyResult struct {
	Token string
	Mask  string
	KeyID uuid.UUID
}

// EnsureServiceMcpAPIKey creates or rotates the MCP-dedicated API Key for a service.
// Plaintext token is returned only on create/rotate; otherwise only Mask is set.
func (s *ServiceLayer) EnsureServiceMcpAPIKey(
	ctx context.Context,
	userID uuid.UUID,
	projectID, serviceID uuid.UUID,
	svc *Service,
	apiKeys *apikey.Service,
	regenerate bool,
) (McpAPIKeyResult, *Service, error) {
	var zero McpAPIKeyResult
	if apiKeys == nil {
		return zero, svc, errors.New("api key service is not configured")
	}
	if userID == uuid.Nil {
		return zero, svc, errors.New("user id is required")
	}
	if svc == nil {
		return zero, nil, errors.New("service is required")
	}

	keyName := fmt.Sprintf("MCP · %s", strings.TrimSpace(svc.Name))
	if keyName == "MCP · " {
		keyName = "MCP · service"
	}

	if svc.McpAPIKeyID != nil && !regenerate {
		key, err := apiKeys.Get(ctx, userID, *svc.McpAPIKeyID)
		if err == nil && key != nil && key.Enabled {
			return McpAPIKeyResult{Mask: key.Mask, KeyID: key.ID}, svc, nil
		}
		// 关联 Key 已删除或不可用，走新建
	}

	if svc.McpAPIKeyID != nil && regenerate {
		_, token, err := apiKeys.Rotate(ctx, userID, *svc.McpAPIKeyID)
		if err != nil {
			if errors.Is(err, apikey.ErrNotFound) {
				svc.McpAPIKeyID = nil
			} else {
				return zero, svc, err
			}
		} else {
			key, _ := apiKeys.Get(ctx, userID, *svc.McpAPIKeyID)
			mask := ""
			if key != nil {
				mask = key.Mask
			}
			return McpAPIKeyResult{Token: token, Mask: mask, KeyID: *svc.McpAPIKeyID}, svc, nil
		}
	}

	created, err := apiKeys.Create(ctx, userID, apikey.CreateInput{
		Name:   keyName,
		Scopes: apikey.MCPAutomationScopes(),
	})
	if err != nil {
		return zero, svc, err
	}

	updated, err := s.repo.SetServiceMcpAPIKeyID(ctx, projectID, serviceID, created.APIKey.ID)
	if err != nil {
		_ = apiKeys.Delete(ctx, userID, created.APIKey.ID)
		return zero, svc, err
	}
	return McpAPIKeyResult{
		Token: created.Token,
		Mask:  created.APIKey.Mask,
		KeyID: created.APIKey.ID,
	}, updated, nil
}
