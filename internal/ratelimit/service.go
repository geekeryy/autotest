package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Service provides rate limiting operations.
type Service struct {
	repo *Repository
}

// NewService constructs a Service.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// Check checks if a request is allowed under the rate limit. If allowed,
// it records the request. Denied requests are NOT recorded, so they don't
// pollute the count for subsequent requests.
func (s *Service) Check(ctx context.Context, userID uuid.UUID, action string) (*CheckResult, error) {
	key := fmt.Sprintf("user:%s:%s", userID.String(), action)
	return s.checkInternal(ctx, key, action)
}

// CheckWithKey checks a custom key (e.g. "ip:10.0.0.1:api") against a rule.
func (s *Service) CheckWithKey(ctx context.Context, key string, ruleName string) (*CheckResult, error) {
	return s.checkInternal(ctx, key, ruleName)
}

func (s *Service) checkInternal(ctx context.Context, key string, ruleName string) (*CheckResult, error) {
	rule, err := s.repo.GetRule(ctx, ruleName)
	if err != nil {
		// No rule or DB error: fail-open
		return &CheckResult{Allowed: true, Limit: 0, Remaining: -1}, nil
	}

	// Count current entries first (no insert yet)
	currentCount, err := s.repo.CountCurrent(ctx, key)
	if err != nil {
		return nil, err
	}

	if currentCount >= rule.Limit {
		// Already at or over limit — deny without inserting
		resetAt, retryAfter := s.computeReset(ctx, key, rule)
		return &CheckResult{
			Allowed:    false,
			Limit:      rule.Limit,
			Remaining:  0,
			ResetAt:    resetAt,
			RetryAfter: retryAfter,
		}, nil
	}

	// Under limit — record the request
	_, err = s.repo.RecordRequest(ctx, key, rule.Window)
	if err != nil {
		return nil, err
	}

	remaining := rule.Limit - currentCount - 1
	if remaining < 0 {
		remaining = 0
	}
	resetAt := time.Now().Add(time.Duration(rule.Window) * time.Second).Unix()

	return &CheckResult{
		Allowed:   true,
		Limit:     rule.Limit,
		Remaining: remaining,
		ResetAt:   resetAt,
	}, nil
}

// computeReset calculates the actual reset time from the earliest entry expiry.
func (s *Service) computeReset(ctx context.Context, key string, rule *RateLimitRule) (int64, int) {
	earliest, err := s.repo.GetEarliestExpiry(ctx, key)
	if err != nil {
		// Fallback to worst case
		now := time.Now()
		return now.Add(time.Duration(rule.Window) * time.Second).Unix(), rule.Window
	}
	retryAfter := int(time.Until(*earliest).Seconds())
	if retryAfter < 1 {
		retryAfter = 1
	}
	return earliest.Unix(), retryAfter
}

// InitPresetRules creates the default rate limit rules if they don't exist.
func (s *Service) InitPresetRules(ctx context.Context) error {
	for _, rule := range PresetRules() {
		_, err := s.repo.CreateRule(ctx, rule)
		if err != nil {
			// Likely UNIQUE violation — rule already exists, ignore
			continue
		}
	}
	return nil
}

// Cleanup removes expired entries. Should be called periodically.
func (s *Service) Cleanup(ctx context.Context) (int64, error) {
	return s.repo.Cleanup(ctx)
}

// ListRules returns all enabled rate limit rules.
func (s *Service) ListRules(ctx context.Context) ([]RateLimitRule, error) {
	return s.repo.ListRules(ctx)
}

// CreateRule creates a new rate limit rule.
func (s *Service) CreateRule(ctx context.Context, rule RateLimitRule) (*RateLimitRule, error) {
	return s.repo.CreateRule(ctx, rule)
}
