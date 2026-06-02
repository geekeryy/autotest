package aimemory

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Service provides memory operations with user/project isolation.
type Service struct {
	repo *Repository
}

// NewService constructs a Service.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// Save stores a user-provided memory.
func (s *Service) Save(ctx context.Context, projectID, userID uuid.UUID, input SaveMemoryInput) (*Memory, error) {
	if projectID == uuid.Nil || userID == uuid.Nil {
		return nil, errors.New("projectId 与 userId 不能为空")
	}
	if strings.TrimSpace(input.Key) == "" {
		return nil, errors.New("key 不能为空")
	}
	if strings.TrimSpace(input.Content) == "" {
		return nil, errors.New("content 不能为空")
	}

	memType := input.Type
	if memType == "" {
		memType = TypeProject
	}
	category := input.Category
	if category == "" {
		category = CategoryGeneral
	}

	createInput := CreateMemoryInput{
		ProjectID:  &projectID,
		UserID:     &userID,
		Type:       memType,
		Category:   category,
		Key:        input.Key,
		Content:    input.Content,
		Source:     SourceUserSave,
		Confidence: 0.9,
	}
	if input.Confidence != nil {
		createInput.Confidence = *input.Confidence
	}

	return s.repo.Create(ctx, createInput)
}

// SaveGlobal stores a global memory (admin only).
func (s *Service) SaveGlobal(ctx context.Context, input SaveMemoryInput) (*Memory, error) {
	if strings.TrimSpace(input.Key) == "" {
		return nil, errors.New("key 不能为空")
	}
	if strings.TrimSpace(input.Content) == "" {
		return nil, errors.New("content 不能为空")
	}

	category := input.Category
	if category == "" {
		category = CategoryGeneral
	}
	createInput := CreateMemoryInput{
		Type:       TypeGlobal,
		Category:   category,
		Key:        input.Key,
		Content:    input.Content,
		Source:     SourceSystem,
		Confidence: 1.0,
	}
	if input.Confidence != nil {
		createInput.Confidence = *input.Confidence
	}

	return s.repo.Create(ctx, createInput)
}

// Recall retrieves relevant memories for a query using keyword matching.
func (s *Service) Recall(ctx context.Context, projectID, userID uuid.UUID, query string, limit int) ([]MemoryRecallResult, error) {
	if projectID == uuid.Nil || userID == uuid.Nil {
		return nil, errors.New("projectId 与 userId 不能为空")
	}
	if limit <= 0 {
		limit = 10
	}

	// List all relevant memories
	allMemories, _, err := s.repo.List(ctx, projectID, userID, ListOptions{Limit: 100})
	if err != nil {
		return nil, err
	}

	// Score memories by keyword relevance
	query = strings.ToLower(strings.TrimSpace(query))
	queryWords := strings.Fields(query)

	scored := []MemoryRecallResult{}
	for _, m := range allMemories {
		score := scoreMemory(m, queryWords)
		if score > 0 {
			scored = append(scored, MemoryRecallResult{
				Memory:    m,
				Relevance: score,
			})
			// Touch the memory to update ref_count
			_ = s.repo.Touch(ctx, m.ID)
		}
	}

	// Sort by relevance (simple bubble sort for small lists)
	for i := 0; i < len(scored); i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[j].Relevance > scored[i].Relevance {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}

	// Limit results
	if len(scored) > limit {
		scored = scored[:limit]
	}

	return scored, nil
}

// scoreMemory calculates a relevance score for a memory against query words.
func scoreMemory(m Memory, queryWords []string) float64 {
	if len(queryWords) == 0 {
		return 0.5 // Default relevance for empty query
	}

	score := 0.0
	content := strings.ToLower(m.Content)
	key := strings.ToLower(m.Key)

	for _, word := range queryWords {
		if strings.Contains(content, word) {
			score += 1.0
		}
		if strings.Contains(key, word) {
			score += 2.0 // Key matches are more important
		}
	}

	// Normalize by number of query words
	score = score / float64(len(queryWords))

	// Apply confidence weight
	score *= m.Confidence

	// Boost by ref_count (logarithmic)
	if m.RefCount > 0 {
		score *= (1.0 + float64(m.RefCount)*0.1)
	}

	return score
}

// List returns memories for a user/project.
func (s *Service) List(ctx context.Context, projectID, userID uuid.UUID, opts ListOptions) ([]Memory, int, error) {
	if projectID == uuid.Nil && userID == uuid.Nil {
		return nil, 0, errors.New("projectId 或 userId 至少需要一个")
	}
	return s.repo.List(ctx, projectID, userID, opts)
}

// Get returns a single memory.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Memory, error) {
	return s.repo.Get(ctx, id)
}

// Update modifies a memory.
func (s *Service) Update(ctx context.Context, id uuid.UUID, input UpdateMemoryInput) (*Memory, error) {
	return s.repo.Update(ctx, id, input)
}

// Delete removes a memory.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// ExtractAndStore extracts memories from a conversation and stores them.
func (s *Service) ExtractAndStore(ctx context.Context, projectID, userID uuid.UUID, messages []StoredMessageForExtract) error {
	if projectID == uuid.Nil || userID == uuid.Nil {
		return errors.New("projectId 与 userId 不能为空")
	}

	// Extract user preferences and patterns
	extracted := extractMemoriesFromMessages(messages)

	for _, m := range extracted {
		input := CreateMemoryInput{
			ProjectID:  &projectID,
			UserID:     &userID,
			Type:       TypeAuto,
			Category:   m.Category,
			Key:        m.Key,
			Content:    m.Content,
			Source:     SourceAutoExtract,
			Confidence: 0.7,
			ExpiresAt:  timePtr(time.Now().Add(30 * 24 * time.Hour)), // 30 days
		}
		if _, err := s.repo.Create(ctx, input); err != nil {
			// Log error but continue with other memories
			continue
		}
	}

	return nil
}

// StoredMessageForExtract is a simplified message for memory extraction.
type StoredMessageForExtract struct {
	Role    string
	Content string
}

// extractableMemory represents a memory to be stored.
type extractableMemory struct {
	Category string
	Key      string
	Content  string
}

// extractMemoriesFromMessages extracts memories from conversation messages.
func extractMemoriesFromMessages(messages []StoredMessageForExtract) []extractableMemory {
	var results []extractableMemory

	for _, m := range messages {
		if m.Role != "user" {
			continue
		}

		content := strings.ToLower(m.Content)

		// Extract user preferences
		if strings.Contains(content, "我喜欢") || strings.Contains(content, "我偏好") || strings.Contains(content, "我习惯") || strings.Contains(content, "不要") {
			results = append(results, extractableMemory{
				Category: CategoryPreference,
				Key:      "user_preference",
				Content:  m.Content,
			})
		}

		// Extract usage patterns
		if strings.Contains(content, "每次都") || strings.Contains(content, "总是") || strings.Contains(content, "通常") || strings.Contains(content, "一般") {
			results = append(results, extractableMemory{
				Category: CategoryPattern,
				Key:      "usage_pattern",
				Content:  m.Content,
			})
		}

		// Extract API conventions
		if strings.Contains(content, "接口") || strings.Contains(content, "api") || strings.Contains(content, "endpoint") {
			if strings.Contains(content, "约定") || strings.Contains(content, "规范") || strings.Contains(content, "必须") || strings.Contains(content, "要求") {
				results = append(results, extractableMemory{
					Category: CategoryConvention,
					Key:      "api_convention",
					Content:  m.Content,
				})
			}
		}

		// Extract assertion patterns
		if strings.Contains(content, "断言") || strings.Contains(content, "校验") || strings.Contains(content, "验证") {
			if strings.Contains(content, "应该") || strings.Contains(content, "需要") || strings.Contains(content, "必须") {
				results = append(results, extractableMemory{
					Category: CategoryAssertion,
					Key:      "assertion_pattern",
					Content:  m.Content,
				})
			}
		}

		// Extract environment/project specific knowledge
		if strings.Contains(content, "环境") || strings.Contains(content, "配置") || strings.Contains(content, "部署") {
			results = append(results, extractableMemory{
				Category: CategoryEnvironment,
				Key:      "env_knowledge",
				Content:  m.Content,
			})
		}
	}

	// Extract from assistant responses (patterns that worked well)
	for _, m := range messages {
		if m.Role != "assistant" {
			continue
		}
		content := strings.ToLower(m.Content)
		// Extract successful test patterns
		if strings.Contains(content, "成功") || strings.Contains(content, "通过") {
			if strings.Contains(content, "场景") || strings.Contains(content, "用例") {
				results = append(results, extractableMemory{
					Category: CategoryPattern,
					Key:      "successful_pattern",
					Content:  truncateString(m.Content, 200),
				})
			}
		}
	}

	return results
}

// truncateString truncates a string to maxLen bytes, ensuring valid UTF-8.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	end := maxLen
	for end > 0 && !utf8.RuneStart(s[end]) {
		end--
	}
	return s[:end] + "..."
}

// Prune removes expired and low-confidence memories.
func (s *Service) Prune(ctx context.Context) (int, error) {
	return s.repo.Prune(ctx, 30*24*time.Hour, 0.3)
}

// FormatMemoriesForPrompt formats memories for injection into the system prompt.
func FormatMemoriesForPrompt(memories []MemoryRecallResult) string {
	if len(memories) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## 历史记忆\n")
	sb.WriteString("以下是从历史对话中提取的相关记忆，可作为参考：\n\n")

	for i, mr := range memories {
		sb.WriteString(fmt.Sprintf("%d. **%s** (相关度: %.1f)\n", i+1, mr.Memory.Key, mr.Relevance))
		sb.WriteString("   " + mr.Memory.Content + "\n\n")
	}

	return sb.String()
}

func timePtr(t time.Time) *time.Time {
	return &t
}
