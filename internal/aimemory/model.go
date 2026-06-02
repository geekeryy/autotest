// Package aimemory implements a multi-layer memory system for the AI assistant.
// It stores user preferences, project conventions, and global best practices
// that persist across sessions.
package aimemory

import (
	"time"

	"github.com/google/uuid"
)

// Memory type constants
const (
	TypeAuto    = "auto"    // Auto-extracted from conversations
	TypeProject = "project" // Project-specific knowledge
	TypeGlobal  = "global"  // Cross-project best practices
)

// Memory category constants
const (
	CategoryGeneral     = "general"
	CategoryPreference  = "preference"
	CategoryPattern     = "pattern"
	CategoryConvention  = "convention"
	CategoryAssertion   = "assertion"
	CategoryEnvironment = "environment"
	CategoryFact        = "fact"
)

// Memory source constants
const (
	SourceAutoExtract = "auto_extract"
	SourceUserSave    = "user_save"
	SourceSystem      = "system"
)

// Memory represents a single memory entry.
type Memory struct {
	ID         uuid.UUID  `json:"id"`
	ProjectID  *uuid.UUID `json:"projectId,omitempty"`
	UserID     *uuid.UUID `json:"userId,omitempty"`
	Type       string     `json:"type"`
	Category   string     `json:"category"`
	Key        string     `json:"key"`
	Content    string     `json:"content"`
	Source     string     `json:"source"`
	Confidence float64    `json:"confidence"`
	RefCount   int        `json:"refCount"`
	LastUsedAt *time.Time `json:"lastUsedAt,omitempty"`
	ExpiresAt  *time.Time `json:"expiresAt,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
	UpdatedAt  time.Time  `json:"updatedAt"`
}

// SaveMemoryInput is the payload for saving a memory.
type SaveMemoryInput struct {
	Type       string `json:"type"`
	Category   string `json:"category"`
	Key        string `json:"key"`
	Content    string `json:"content"`
	Confidence *float64 `json:"confidence,omitempty"`
}

// ListOptions controls memory listing.
type ListOptions struct {
	Type     string `json:"type,omitempty"`
	Category string `json:"category,omitempty"`
	Limit    int    `json:"limit,omitempty"`
	Offset   int    `json:"offset,omitempty"`
}

// MemoryRecallResult is a memory with its relevance score.
type MemoryRecallResult struct {
	Memory    Memory  `json:"memory"`
	Relevance float64 `json:"relevance"`
}
