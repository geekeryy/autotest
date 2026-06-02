package ratelimit

import (
	"time"

	"github.com/google/uuid"
)

// RateLimitRule defines a rate limit configuration.
type RateLimitRule struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Key       string    `json:"key"`       // e.g. "user:123:ai:chat", "ip:10.0.0.1:api"
	Limit     int       `json:"limit"`     // max requests per window
	Window    int       `json:"window"`    // window duration in seconds
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// RateLimitEntry tracks a single request in the sliding window.
type RateLimitEntry struct {
	ID        uuid.UUID `json:"id"`
	Key       string    `json:"key"`
	CreatedAt time.Time `json:"createdAt"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// CheckResult is the result of a rate limit check.
type CheckResult struct {
	Allowed    bool  `json:"allowed"`
	Limit      int   `json:"limit"`
	Remaining  int   `json:"remaining"`
	ResetAt    int64 `json:"resetAt"`    // unix timestamp when window resets
	RetryAfter int   `json:"retryAfter"` // seconds to wait if denied
}

// PresetRules returns the default rate limit rules for AI operations.
func PresetRules() []RateLimitRule {
	return []RateLimitRule{
		{Name: "ai_chat_per_user", Key: "ai:chat", Limit: 60, Window: 60},
		{Name: "ai_generate_per_user", Key: "ai:generate", Limit: 30, Window: 60},
		{Name: "ai_analyze_per_user", Key: "ai:analyze", Limit: 30, Window: 60},
		{Name: "ai_scenario_gen_per_user", Key: "ai:scenario_gen", Limit: 10, Window: 60},
		{Name: "ai_background_per_user", Key: "ai:background", Limit: 5, Window: 300},
	}
}
