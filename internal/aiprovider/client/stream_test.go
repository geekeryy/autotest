package client

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestParseOpenAIStream_TextOnly verifies the common case: a sequence of
// text chunks terminated by `data: [DONE]` produces text events and a
// final done event with the captured finish_reason.
func TestParseOpenAIStream_TextOnly(t *testing.T) {
	stream := strings.Join([]string{
		`data: {"choices":[{"index":0,"delta":{"role":"assistant","content":""}}]}`,
		`data: {"choices":[{"index":0,"delta":{"content":"Hello"}}]}`,
		`data: {"choices":[{"index":0,"delta":{"content":" world"}}]}`,
		`data: {"choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"model":"gpt-4o-mini"}`,
		`data: [DONE]`,
		"",
	}, "\n\n")

	var (
		texts  []string
		finish string
		model  string
		tools  []ToolCall
	)
	err := parseOpenAIStream(strings.NewReader(stream), "fallback", func(ev StreamEvent) error {
		switch ev.Kind {
		case StreamEventText:
			texts = append(texts, ev.Text)
		case StreamEventToolCall:
			if ev.ToolCall != nil {
				tools = append(tools, *ev.ToolCall)
			}
		case StreamEventDone:
			finish = ev.Finish
			model = ev.Model
		}
		return nil
	})
	if err != nil {
		t.Fatalf("parseOpenAIStream returned error: %v", err)
	}
	if got := strings.Join(texts, ""); got != "Hello world" {
		t.Fatalf("unexpected joined text: %q", got)
	}
	if finish != "stop" {
		t.Fatalf("unexpected finish reason: %q", finish)
	}
	if model != "gpt-4o-mini" {
		t.Fatalf("unexpected model: %q", model)
	}
	if len(tools) != 0 {
		t.Fatalf("expected no tool calls, got %d", len(tools))
	}
}

func TestParseOpenAIStream_ReasoningContent(t *testing.T) {
	stream := strings.Join([]string{
		`data: {"choices":[{"index":0,"delta":{"reasoning_content":"先分析"}}]}`,
		`data: {"choices":[{"index":0,"delta":{"reasoning_content":"问题","content":"结论"}}]}`,
		`data: {"choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
		`data: [DONE]`,
		"",
	}, "\n\n")

	var reasoning []string
	var texts []string
	err := parseOpenAIStream(strings.NewReader(stream), "fallback", func(ev StreamEvent) error {
		switch ev.Kind {
		case StreamEventReasoning:
			reasoning = append(reasoning, ev.ReasoningContent)
		case StreamEventText:
			texts = append(texts, ev.Text)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("parseOpenAIStream returned error: %v", err)
	}
	if got := strings.Join(reasoning, ""); got != "先分析问题" {
		t.Fatalf("unexpected reasoning content: %q", got)
	}
	if got := strings.Join(texts, ""); got != "结论" {
		t.Fatalf("unexpected visible text: %q", got)
	}
}

// TestParseOpenAIStream_ToolCallReassembly checks that an interleaved
// tool-call stream (arguments arriving across multiple chunks) is
// reassembled into a single StreamEventToolCall with the full JSON
// preserved. We build the chunks programmatically rather than as raw
// strings to avoid double-escaping bugs in the test fixtures.
func TestParseOpenAIStream_ToolCallReassembly(t *testing.T) {
	frame := func(value any) string {
		body, err := json.Marshal(value)
		if err != nil {
			t.Fatalf("marshal frame: %v", err)
		}
		return "data: " + string(body)
	}
	chunks := []string{
		frame(map[string]any{
			"choices": []any{map[string]any{
				"index": 0,
				"delta": map[string]any{"role": "assistant", "content": ""},
			}},
		}),
		frame(map[string]any{
			"choices": []any{map[string]any{
				"index": 0,
				"delta": map[string]any{"tool_calls": []any{map[string]any{
					"index":    0,
					"id":       "call_abc",
					"type":     "function",
					"function": map[string]any{"name": "get_case", "arguments": ""},
				}}},
			}},
		}),
		frame(map[string]any{
			"choices": []any{map[string]any{
				"index": 0,
				"delta": map[string]any{"tool_calls": []any{map[string]any{
					"index":    0,
					"function": map[string]any{"arguments": `{"caseId":`},
				}}},
			}},
		}),
		frame(map[string]any{
			"choices": []any{map[string]any{
				"index": 0,
				"delta": map[string]any{"tool_calls": []any{map[string]any{
					"index":    0,
					"function": map[string]any{"arguments": `"abc"}`},
				}}},
			}},
		}),
		frame(map[string]any{
			"choices": []any{map[string]any{
				"index":         0,
				"delta":         map[string]any{},
				"finish_reason": "tool_calls",
			}},
			"model": "gpt-4o",
		}),
		`data: [DONE]`,
		"",
	}
	stream := strings.Join(chunks, "\n\n")

	var tools []ToolCall
	var finish string
	err := parseOpenAIStream(strings.NewReader(stream), "fallback", func(ev StreamEvent) error {
		switch ev.Kind {
		case StreamEventToolCall:
			if ev.ToolCall != nil {
				tools = append(tools, *ev.ToolCall)
			}
		case StreamEventDone:
			finish = ev.Finish
		}
		return nil
	})
	if err != nil {
		t.Fatalf("parseOpenAIStream returned error: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(tools))
	}
	if tools[0].ID != "call_abc" || tools[0].Name != "get_case" {
		t.Fatalf("unexpected tool call header: %+v", tools[0])
	}
	var args map[string]string
	if err := json.Unmarshal(tools[0].Arguments, &args); err != nil {
		t.Fatalf("arguments not valid JSON (%s): %v", tools[0].Arguments, err)
	}
	if args["caseId"] != "abc" {
		t.Fatalf("expected caseId=abc, got %+v", args)
	}
	if finish != "tool_calls" {
		t.Fatalf("expected finish=tool_calls, got %q", finish)
	}
}

// TestParseAnthropicStream_TextAndTool verifies the content-block state
// machine: one text block followed by one tool_use block whose arguments
// are split across input_json_delta frames produce a text delta sequence
// plus a single tool_use ToolCall.
func TestParseAnthropicStream_TextAndTool(t *testing.T) {
	stream := strings.Join([]string{
		`data: {"type":"message_start","message":{"id":"m1","model":"claude-3-5-sonnet"}}`,
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"分析"}}`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"中"}}`,
		`data: {"type":"content_block_stop","index":0}`,
		`data: {"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"toolu_xx","name":"get_case","input":{}}}`,
		`data: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"caseId\":"}}`,
		`data: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"\"abc\"}"}}`,
		`data: {"type":"content_block_stop","index":1}`,
		`data: {"type":"message_delta","delta":{"stop_reason":"tool_use"}}`,
		`data: {"type":"message_stop"}`,
		"",
	}, "\n\n")

	var (
		texts  []string
		tools  []ToolCall
		finish string
		model  string
	)
	err := parseAnthropicStream(strings.NewReader(stream), "fallback", func(ev StreamEvent) error {
		switch ev.Kind {
		case StreamEventText:
			texts = append(texts, ev.Text)
		case StreamEventToolCall:
			if ev.ToolCall != nil {
				tools = append(tools, *ev.ToolCall)
			}
		case StreamEventDone:
			finish = ev.Finish
			model = ev.Model
		}
		return nil
	})
	if err != nil {
		t.Fatalf("parseAnthropicStream returned error: %v", err)
	}
	if got := strings.Join(texts, ""); got != "分析中" {
		t.Fatalf("unexpected joined text: %q", got)
	}
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(tools))
	}
	if tools[0].ID != "toolu_xx" || tools[0].Name != "get_case" {
		t.Fatalf("unexpected tool call header: %+v", tools[0])
	}
	var args map[string]string
	if err := json.Unmarshal(tools[0].Arguments, &args); err != nil {
		t.Fatalf("arguments not valid JSON (%s): %v", tools[0].Arguments, err)
	}
	if args["caseId"] != "abc" {
		t.Fatalf("expected caseId=abc, got %+v", args)
	}
	if finish != "tool_use" {
		t.Fatalf("expected finish=tool_use, got %q", finish)
	}
	if model != "claude-3-5-sonnet" {
		t.Fatalf("expected model claude-3-5-sonnet, got %q", model)
	}
}

func TestParseOpenAIStream_UsageOnDone(t *testing.T) {
	stream := strings.Join([]string{
		`data: {"choices":[{"index":0,"delta":{"content":"hi"}}]}`,
		`data: {"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15,"prompt_tokens_details":{"cached_tokens":3}}}`,
		`data: [DONE]`,
		"",
	}, "\n\n")
	var doneUsage *TokenUsage
	_ = parseOpenAIStream(strings.NewReader(stream), "m", func(ev StreamEvent) error {
		if ev.Kind == StreamEventDone && ev.Usage != nil {
			u := *ev.Usage
			doneUsage = &u
		}
		return nil
	})
	if doneUsage == nil || doneUsage.PromptTokens != 10 || doneUsage.CachedPromptTokens != 3 {
		t.Fatalf("usage: %+v", doneUsage)
	}
}

// TestParseOpenAIStream_CallbackAbort ensures a callback returning an
// error stops the stream and the resulting error is surfaced from
// parseOpenAIStream. A final done event is still emitted so the SSE
// handler can clean up.
func TestParseOpenAIStream_CallbackAbort(t *testing.T) {
	stream := strings.Join([]string{
		`data: {"choices":[{"index":0,"delta":{"content":"a"}}]}`,
		`data: {"choices":[{"index":0,"delta":{"content":"b"}}]}`,
		`data: [DONE]`,
		"",
	}, "\n\n")

	calls := 0
	saw := struct {
		done bool
	}{}
	err := parseOpenAIStream(strings.NewReader(stream), "x", func(ev StreamEvent) error {
		calls++
		if ev.Kind == StreamEventDone {
			saw.done = true
			return nil
		}
		if calls == 1 {
			return errCallbackTest
		}
		return nil
	})
	if err != errCallbackTest {
		t.Fatalf("expected sentinel error, got %v", err)
	}
	if !saw.done {
		t.Fatalf("expected a final done event")
	}
}

// errCallbackTest is a sentinel to make TestParseOpenAIStream_CallbackAbort
// readable and to avoid string-comparing errors.
var errCallbackTest = stringError("callback aborted")

type stringError string

func (e stringError) Error() string { return string(e) }
