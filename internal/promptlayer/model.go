// Package promptlayer manages versioned prompt layers for the AI assistant.
// It supports layered prompt assembly with version control and rollback.
package promptlayer

import (
	"time"

	"github.com/google/uuid"
)

// PromptLayer represents a versioned prompt layer.
type PromptLayer struct {
	ID        uuid.UUID  `json:"id"`
	Name      string     `json:"name"`
	Domain    string     `json:"domain"`
	Content   string     `json:"content"`
	Version   int        `json:"version"`
	Enabled   bool       `json:"enabled"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
}

// CreateLayerInput is the payload for creating a prompt layer.
type CreateLayerInput struct {
	Name    string `json:"name"`
	Domain  string `json:"domain"`
	Content string `json:"content"`
}

// UpdateLayerInput is the payload for updating a prompt layer.
type UpdateLayerInput struct {
	Content *string `json:"content,omitempty"`
	Enabled *bool   `json:"enabled,omitempty"`
}

// ListLayersOptions controls layer listing.
type ListLayersOptions struct {
	Domain  string `json:"domain,omitempty"`
	Enabled *bool  `json:"enabled,omitempty"`
	Limit   int    `json:"limit,omitempty"`
	Offset  int    `json:"offset,omitempty"`
}

// PromptExperiment represents an A/B test between two prompt layer versions.
type PromptExperiment struct {
	ID           uuid.UUID  `json:"id"`
	Domain       string     `json:"domain"`
	LayerName    string     `json:"layerName"`
	VariantAID   *uuid.UUID `json:"variantAId,omitempty"`
	VariantBID   *uuid.UUID `json:"variantBId,omitempty"`
	TrafficSplit float64    `json:"trafficSplit"` // 0.0-1.0, traffic proportion for variant A
	Metric       string     `json:"metric"`       // success_rate / avg_hops / user_rating
	VariantAVal  float64    `json:"variantAVal"`
	VariantBVal  float64    `json:"variantBVal"`
	Status       string     `json:"status"` // running / completed / stopped
	StartedAt    time.Time  `json:"startedAt"`
	EndedAt      *time.Time `json:"endedAt,omitempty"`
}

// CreateExperimentInput is the payload for creating an experiment.
type CreateExperimentInput struct {
	Domain       string  `json:"domain"`
	LayerName    string  `json:"layerName"`
	VariantAID   string  `json:"variantAId"`
	VariantBID   string  `json:"variantBId"`
	TrafficSplit float64 `json:"trafficSplit"`
	Metric       string  `json:"metric"`
}
