package aiprovider

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"autotest/internal/aiprovider/client"

	"github.com/google/uuid"
)

func TestShouldAutoSessionTitle(t *testing.T) {
	cases := []struct {
		title string
		want  bool
	}{
		{"", true},
		{"新会话", true},
		{"帮我生成登录场景…", true},
		{"用户登录下单流程", false},
		{"自定义标题", false},
	}
	for _, tc := range cases {
		if got := shouldAutoSessionTitle(tc.title); got != tc.want {
			t.Errorf("shouldAutoSessionTitle(%q) = %v, want %v", tc.title, got, tc.want)
		}
	}
}

func TestShouldScheduleSessionTitle(t *testing.T) {
	if !shouldScheduleSessionTitle(1, "") {
		t.Fatal("1st turn with empty title")
	}
	if !shouldScheduleSessionTitle(2, "查询登录接口") {
		t.Fatal("2nd turn should refine existing auto title")
	}
	if shouldScheduleSessionTitle(3, "") {
		t.Fatal("3rd turn must not update title")
	}
	if !shouldScheduleSessionTitle(2, "首轮生成的标题") {
		t.Fatal("2nd turn should refine title set on 1st turn")
	}
}

func TestFormatSessionTitleUserPrompt(t *testing.T) {
	got := formatSessionTitleUserPrompt("创建场景", "补充断言步骤")
	if got != "创建场景\n---\n补充断言步骤" {
		t.Fatalf("got %q", got)
	}
	if formatSessionTitleUserPrompt("仅首问", "") != "仅首问" {
		t.Fatal("single question should pass through unchanged")
	}
}

func TestSanitizeSessionTitle(t *testing.T) {
	if got := sanitizeSessionTitle(`  "查询接口列表"  `); got != "查询接口列表" {
		t.Fatalf("got %q", got)
	}
	if got := sanitizeSessionTitle("首先，用户的指令是创建测试场景"); got != "创建测试场景" {
		t.Fatalf("meta preamble strip, got %q", got)
	}
	if got := sanitizeSessionTitle("标题：登录接口用例查询"); got != "登录接口用例查询" {
		t.Fatalf("label prefix strip, got %q", got)
	}
	if got := sanitizeSessionTitle("首先，用户的指令是……"); got != "" {
		t.Fatalf("still meta after strip should be empty, got %q", got)
	}
	long := sanitizeSessionTitle("这是一个非常非常非常非常非常非常长的会话标题应该被截断")
	if len([]rune(long)) > maxSessionTitleRunes {
		t.Fatalf("expected truncation to %d runes, got %d", maxSessionTitleRunes, len([]rune(long)))
	}
}

func TestChatResultTitleText_IgnoresReasoning(t *testing.T) {
	if got := chatResultTitleText(&client.Result{Text: "  标题  "}); got != "标题" {
		t.Fatalf("got %q", got)
	}
	if got := chatResultTitleText(&client.Result{ReasoningContent: "首先，用户的指令是创建场景"}); got != "" {
		t.Fatalf("reasoning must not be used as title, got %q", got)
	}
}

type mockTitleChatClient struct {
	lastOpts     client.Options
	lastUserBody string
	result       *client.Result
	err          error
}

func (m *mockTitleChatClient) Chat(ctx context.Context, messages []client.Message, opts client.Options) (*client.Result, error) {
	m.lastOpts = opts
	for _, msg := range messages {
		if msg.Role == "user" {
			m.lastUserBody = msg.Content
		}
	}
	return m.result, m.err
}

func TestCallSessionTitleModel_DisablesThinking(t *testing.T) {
	cli := &mockTitleChatClient{result: &client.Result{Text: "创建测试场景"}}
	cfg := &streamConfig{
		Cli:      cli,
		BaseOpts: client.Options{Model: "deepseek-chat"},
		Provider: &providerRow{DefaultModel: "deepseek-chat", ProviderType: ProviderTypeDeepSeek},
	}
	title, err := new(Service).callSessionTitleModel(context.Background(), cfg, "如何创建场景", "")
	if err != nil {
		t.Fatal(err)
	}
	if title != "创建测试场景" {
		t.Fatalf("got title %q", title)
	}
	if cli.lastOpts.Thinking == nil || cli.lastOpts.Thinking.Enabled {
		t.Fatalf("title call must disable thinking, got %+v", cli.lastOpts.Thinking)
	}
	if cli.lastUserBody != "如何创建场景" {
		t.Fatalf("user prompt should be raw question, got %q", cli.lastUserBody)
	}
}

func TestGenerateSessionTitle_FirstTurn(t *testing.T) {
	sessionID := uuid.New()
	projectID := uuid.New()
	userID := uuid.New()
	var sinkEvents []AssistantStreamEvent
	store := &fakeSessionStore{
		session: StoredSession{ID: sessionID, Title: ""},
		messages: []StoredMessage{
			{Role: "user", Content: "如何创建场景"},
		},
	}
	cli := &mockTitleChatClient{result: &client.Result{Text: "创建测试场景"}}
	svc := &Service{}
	cfg := &streamConfig{
		ProjectID: projectID,
		SessionID: sessionID,
		UserID:    userID,
		Cli:       cli,
		Store:     store,
		Sink: func(ev AssistantStreamEvent) error {
			sinkEvents = append(sinkEvents, ev)
			return nil
		},
	}
	if err := svc.generateSessionTitle(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	if store.session.Title != "创建测试场景" {
		t.Fatalf("expected persisted title, got %q", store.session.Title)
	}
	if len(sinkEvents) != 1 || sinkEvents[0].Kind != StreamEventSession {
		t.Fatalf("unexpected sink events: %+v", sinkEvents)
	}
}

func TestGenerateSessionTitle_SecondTurnRefines(t *testing.T) {
	sessionID := uuid.New()
	store := &fakeSessionStore{
		session: StoredSession{ID: sessionID, Title: "创建场景"},
		messages: []StoredMessage{
			{Role: "user", Content: "如何创建场景"},
			{Role: "assistant", Content: "…"},
			{Role: "user", Content: "要包含登录步骤"},
		},
	}
	cli := &mockTitleChatClient{result: &client.Result{Text: "创建含登录的场景"}}
	svc := &Service{}
	cfg := &streamConfig{
		ProjectID: uuid.New(),
		SessionID: sessionID,
		UserID:    uuid.New(),
		Cli:       cli,
		Store:     store,
		Sink:      func(AssistantStreamEvent) error { return nil },
	}
	if err := svc.generateSessionTitle(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	if store.session.Title != "创建含登录的场景" {
		t.Fatalf("got %q", store.session.Title)
	}
	if !strings.Contains(cli.lastUserBody, "要包含登录步骤") {
		t.Fatalf("secondary question missing from prompt: %q", cli.lastUserBody)
	}
}

func TestGenerateSessionTitle_SkipsThirdTurn(t *testing.T) {
	store := &fakeSessionStore{
		session: StoredSession{Title: ""},
		messages: []StoredMessage{
			{Role: "user", Content: "a"},
			{Role: "user", Content: "b"},
			{Role: "user", Content: "c"},
		},
	}
	svc := &Service{}
	cfg := &streamConfig{
		ProjectID: uuid.New(),
		SessionID: uuid.New(),
		UserID:    uuid.New(),
		Cli:       &mockTitleChatClient{result: &client.Result{Text: "不应调用"}},
		Store:     store,
		Sink:      func(AssistantStreamEvent) error { return nil },
	}
	if err := svc.generateSessionTitle(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	if store.session.Title != "" {
		t.Fatalf("expected no title update on 3rd turn, got %q", store.session.Title)
	}
}

type fakeSessionStore struct {
	session  StoredSession
	messages []StoredMessage
}

func (f *fakeSessionStore) ListMessages(context.Context, uuid.UUID) ([]StoredMessage, error) {
	return f.messages, nil
}
func (f *fakeSessionStore) AppendMessage(context.Context, uuid.UUID, StoredMessageInput) (*StoredMessage, error) {
	return nil, nil
}
func (f *fakeSessionStore) FindPendingToolCall(context.Context, uuid.UUID, string) (*StoredMessage, *StoredToolCall, error) {
	return nil, nil, nil
}
func (f *fakeSessionStore) MarkPendingFinal(context.Context, uuid.UUID, json.RawMessage) error {
	return nil
}
func (f *fakeSessionStore) MarkPendingRejected(context.Context, uuid.UUID) error { return nil }
func (f *fakeSessionStore) GetSession(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (*StoredSession, error) {
	return &f.session, nil
}
func (f *fakeSessionStore) UpdateSessionTitle(_ context.Context, _, _, _ uuid.UUID, title string) (*StoredSession, error) {
	f.session.Title = title
	return &f.session, nil
}
