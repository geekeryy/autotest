package aiprovider

import (
	"context"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"autotest/internal/aiprovider/client"

	"github.com/google/uuid"
)

// Few-shot title prompt (ChatGPT / Claude style): examples teach “title only”,
// no chain-of-thought or instruction analysis in the output.
const sessionTitleSystem = `根据用户提问生成会话标题。你只能输出标题本身一行，6～18 个汉字，紧扣用户意图、用词具体。

示例（只输出右侧标题，不要复述左侧）：
问：帮我查登录接口有哪些测试用例
标题：登录接口用例查询

问：如何给场景加一个带断言的 HTTP 步骤？
标题：场景添加 HTTP 断言步骤

问：列出项目里失败的最近一次运行
标题：最近失败运行查询

规则：
- 第一次提问为主，第二次提问（若有）仅作辅助，不得喧宾夺主
- 禁止输出分析过程，禁止以「首先」「用户的指令」「根据用户」等开头
- 禁止引号、markdown、编号、冒号前缀（如「标题：」）及任何解释句`

const maxSessionTitleRunes = 18

// StoredSession is minimal session metadata exposed through SessionStore.
type StoredSession struct {
	ID    uuid.UUID `json:"id"`
	Title string    `json:"title"`
}

// scheduleSessionTitleIfNeeded starts async title generation when the session
// is still within the first two user turns. The returned channel is closed
// when generation finishes (or is skipped). Callers must wait on it before
// closing the SSE response so the session event can reach the client.
func closedDoneChan() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}

func (s *Service) scheduleSessionTitleIfNeeded(ctx context.Context, cfg *streamConfig) <-chan struct{} {
	done := make(chan struct{})
	if cfg.Store == nil || cfg.Sink == nil || cfg.SessionID == uuid.Nil {
		close(done)
		return done
	}
	go func() {
		defer close(done)
		bgCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 20*time.Second)
		defer cancel()
		_ = s.generateSessionTitle(bgCtx, cfg)
	}()
	return done
}

func (s *Service) generateSessionTitle(ctx context.Context, cfg *streamConfig) error {
	session, err := cfg.Store.GetSession(ctx, cfg.ProjectID, cfg.UserID, cfg.SessionID)
	if err != nil {
		return err
	}
	userCount, userMsgs := userMessageSnapshot(ctx, cfg.Store, cfg.SessionID, 2)
	if userCount < 1 || len(userMsgs) == 0 {
		return nil
	}
	if !shouldScheduleSessionTitle(userCount, session.Title) {
		return nil
	}

	primary := userMsgs[0]
	secondary := ""
	if len(userMsgs) >= 2 {
		secondary = userMsgs[1]
	}

	title, err := s.callSessionTitleModel(ctx, cfg, primary, secondary)
	if title == "" || looksLikeMetaSessionTitle(title) {
		title = sanitizeSessionTitle(fallbackSessionTitle(primary, secondary))
	}
	if title == "" {
		return err
	}

	updated, err := cfg.Store.UpdateSessionTitle(ctx, cfg.ProjectID, cfg.UserID, cfg.SessionID, title)
	if err != nil {
		return err
	}
	return cfg.Sink(AssistantStreamEvent{
		Kind:    StreamEventSession,
		Session: updated,
	})
}

func (s *Service) callSessionTitleModel(ctx context.Context, cfg *streamConfig, primary, secondary string) (string, error) {
	if cfg.Cli == nil {
		return "", nil
	}
	model := strings.TrimSpace(cfg.BaseOpts.Model)
	if model == "" && cfg.Provider != nil {
		model = strings.TrimSpace(cfg.Provider.DefaultModel)
	}
	temp := 0.1
	// Title must be a short plain-text label. Disable provider "thinking"
	// (DeepSeek / Xiaomi, etc.): with thinking on, many models return an empty
	// content field and put everything in reasoning_content instead.
	thinkingOff := &client.OpenAIThinkingConfig{Enabled: false}
	result, err := cfg.Cli.Chat(ctx, []client.Message{
		{Role: "system", Content: sessionTitleSystem},
		{Role: "user", Content: formatSessionTitleUserPrompt(primary, secondary)},
	}, client.Options{
		Model:       model,
		MaxTokens:   64,
		Temperature: &temp,
		Thinking:    thinkingOff,
	})
	if err != nil {
		return "", err
	}
	return sanitizeSessionTitle(chatResultTitleText(result)), nil
}

func formatSessionTitleUserPrompt(primary, secondary string) string {
	primary = strings.TrimSpace(primary)
	secondary = strings.TrimSpace(secondary)
	if secondary == "" {
		return primary
	}
	return primary + "\n---\n" + secondary
}

// chatResultTitleText uses assistant content only. Reasoning chains must never
// become session titles (they often start with “首先，用户的指令是…”).
func chatResultTitleText(result *client.Result) string {
	if result == nil {
		return ""
	}
	return strings.TrimSpace(result.Text)
}

// userMessageSnapshot returns the total non-empty user message count and up to
// max earliest user contents (in conversation order).
func userMessageSnapshot(ctx context.Context, store SessionStore, sessionID uuid.UUID, max int) (int, []string) {
	msgs, err := store.ListMessages(ctx, sessionID)
	if err != nil || max < 1 {
		return 0, nil
	}
	total := 0
	out := make([]string, 0, max)
	for _, m := range msgs {
		if m.Role != "user" {
			continue
		}
		content := strings.TrimSpace(m.Content)
		if content == "" {
			continue
		}
		total++
		if len(out) < max {
			out = append(out, content)
		}
	}
	return total, out
}

// shouldScheduleSessionTitle reports whether this user turn should trigger
// auto title generation. Only the 1st and 2nd user messages may update the
// title; the 3rd and later never do.
func shouldScheduleSessionTitle(userMsgCount int, title string) bool {
	if userMsgCount < 1 || userMsgCount > 2 {
		return false
	}
	if shouldAutoSessionTitle(title) {
		return true
	}
	// Allow one refinement on the 2nd user turn after a title was set on the 1st.
	return userMsgCount == 2
}

func shouldAutoSessionTitle(title string) bool {
	t := strings.TrimSpace(title)
	if t == "" {
		return true
	}
	switch t {
	case "新会话", "未命名会话", "新对话", "未命名":
		return true
	}
	if strings.HasSuffix(t, "…") || strings.HasSuffix(t, "...") {
		return utf8.RuneCountInString(t) <= 28
	}
	return false
}

func fallbackSessionTitle(primary, secondary string) string {
	if t := truncateUserQuestion(primary); t != "" {
		return t
	}
	return truncateUserQuestion(secondary)
}

func truncateUserQuestion(userMessage string) string {
	s := strings.TrimSpace(userMessage)
	if s == "" {
		return ""
	}
	runes := []rune(s)
	const max = 18
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "…"
}

var (
	sessionTitleLabelRE = regexp.MustCompile(`(?i)^(?:标题|会话标题|对话标题|title)\s*[:：]\s*(.+)$`)
	sessionTitleQuoteRE = regexp.MustCompile(`^[「『"'](.+)[」』"']$`)
)

func sanitizeSessionTitle(raw string) string {
	s := stripSessionTitlePreamble(strings.TrimSpace(raw))
	s = strings.Trim(s, `"'「」『』`)
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	if m := sessionTitleLabelRE.FindStringSubmatch(s); len(m) == 2 {
		s = strings.TrimSpace(m[1])
	}
	if m := sessionTitleQuoteRE.FindStringSubmatch(s); len(m) == 2 {
		s = strings.TrimSpace(m[1])
	}
	if idx := strings.IndexAny(s, "。！？!?"); idx > 0 {
		s = strings.TrimSpace(s[:idx])
	}
	s = strings.TrimSpace(s)
	if s == "" || looksLikeMetaSessionTitle(s) || !hasMeaningfulTitleContent(s) {
		return ""
	}
	if utf8.RuneCountInString(s) > maxSessionTitleRunes {
		runes := []rune(s)
		s = string(runes[:maxSessionTitleRunes])
	}
	return s
}

func hasMeaningfulTitleContent(s string) bool {
	for _, r := range s {
		switch r {
		case ' ', '\t', '.', '…', '。', '，', ',', '、', ':', '：', '-', '—', '"', '\'', '「', '」', '『', '』':
			continue
		default:
			return true
		}
	}
	return false
}

func looksLikeMetaSessionTitle(s string) bool {
	t := strings.TrimSpace(s)
	if t == "" {
		return true
	}
	metaPrefixes := []string{
		"首先", "其次", "然后", "接下来",
		"用户的指令", "用户的问题是", "用户想要", "用户希望", "用户询问", "用户提到",
		"根据用户", "根据第一次", "根据第二次", "根据提问",
		"我需要", "我来", "让我", "应该",
		"标题是", "标题为", "会话标题", "对话标题",
		"the user", "based on", "i need to", "let me",
	}
	for _, p := range metaPrefixes {
		if strings.HasPrefix(t, p) {
			return true
		}
	}
	return false
}

func stripSessionTitlePreamble(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	// Prefer the last non-empty line (models sometimes append the title after analysis).
	if strings.Contains(s, "\n") {
		lines := strings.Split(s, "\n")
		for i := len(lines) - 1; i >= 0; i-- {
			line := strings.TrimSpace(lines[i])
			if line == "" || looksLikeMetaSessionTitle(line) {
				continue
			}
			if m := sessionTitleLabelRE.FindStringSubmatch(line); len(m) == 2 {
				return strings.TrimSpace(m[1])
			}
			return line
		}
	}

	cutMarkers := []string{
		"用户的指令是", "用户的问题是", "用户想要", "用户希望",
		"核心意图是", "意图是", "主题是",
	}
	for _, marker := range cutMarkers {
		if idx := strings.Index(s, marker); idx >= 0 {
			rest := strings.TrimSpace(s[idx+len(marker):])
			rest = strings.TrimLeft(rest, "：:")
			if rest != "" && !looksLikeMetaSessionTitle(rest) {
				return rest
			}
		}
	}

	// Strip leading discourse connectors iteratively.
	for {
		trimmed := s
		for _, prefix := range []string{
			"首先，", "首先,", "首先 ", "其次，", "然后，",
			"根据用户的提问，", "根据用户提问，", "根据用户的问题，",
		} {
			if strings.HasPrefix(trimmed, prefix) {
				trimmed = strings.TrimSpace(trimmed[len(prefix):])
			}
		}
		if trimmed == s {
			break
		}
		s = trimmed
	}
	return s
}
