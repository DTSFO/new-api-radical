package dto

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
)

func oldSearchToolNameByToolCallID(req *ClaudeRequest, toolCallID string) string {
	for _, message := range req.Messages {
		content, _ := common.Any2Type[[]ClaudeMediaMessage](message.Content)
		for _, mediaMessage := range content {
			if mediaMessage.Id == toolCallID {
				return mediaMessage.Name
			}
		}
	}
	return ""
}

func TestClaudeRequestSearchToolNameByToolCallId(t *testing.T) {
	req := &ClaudeRequest{
		Messages: []ClaudeMessage{
			{
				Role: "assistant",
				Content: []any{
					map[string]any{
						"type": "text",
						"text": "hello",
					},
				},
			},
			{
				Role: "assistant",
				Content: []any{
					map[string]any{
						"type": "tool_use",
						"id":   "call_1",
						"name": "weather",
					},
					map[string]any{
						"type": "tool_use",
						"id":   "call_2",
						"name": "search",
					},
				},
			},
			{
				Role: "user",
				Content: []any{
					map[string]any{
						"type":        "tool_result",
						"tool_use_id": "call_2",
						"content": []any{
							map[string]any{
								"type": "text",
								"text": "ok",
							},
						},
					},
				},
			},
		},
	}

	if got := req.SearchToolNameByToolCallId(""); got != "" {
		t.Fatalf("expected empty tool name for empty id, got %q", got)
	}
	if got := req.SearchToolNameByToolCallId("call_1"); got != "weather" {
		t.Fatalf("expected weather, got %q", got)
	}
	if got := req.SearchToolNameByToolCallId("call_2"); got != "search" {
		t.Fatalf("expected search, got %q", got)
	}
	if got := req.SearchToolNameByToolCallId("missing"); got != "" {
		t.Fatalf("expected empty tool name for missing id, got %q", got)
	}
	if got := req.SearchToolNameByToolCallId("call_2"); got != "search" {
		t.Fatalf("expected repeated lookup to stay stable, got %q", got)
	}
}

func TestClaudeParseFastPaths(t *testing.T) {
	text := "hello"
	req := &ClaudeRequest{
		System: []any{
			map[string]any{
				"type": "text",
				"text": "system",
			},
		},
	}
	message := &ClaudeMessage{
		Content: []ClaudeMediaMessage{
			{
				Type: "text",
				Text: &text,
			},
		},
	}
	mediaMessage := &ClaudeMediaMessage{
		Content: []any{
			map[string]any{
				"type": "text",
				"text": "nested",
			},
		},
	}

	parsedContent, err := message.ParseContent()
	if err != nil {
		t.Fatalf("expected ParseContent to succeed, got %v", err)
	}
	if len(parsedContent) != 1 || parsedContent[0].GetText() != "hello" {
		t.Fatalf("unexpected parsed content: %#v", parsedContent)
	}

	parsedSystem := req.ParseSystem()
	if len(parsedSystem) != 1 || parsedSystem[0].GetText() != "system" {
		t.Fatalf("unexpected parsed system: %#v", parsedSystem)
	}

	parsedMedia := mediaMessage.ParseMediaContent()
	if len(parsedMedia) != 1 || parsedMedia[0].GetText() != "nested" {
		t.Fatalf("unexpected parsed media content: %#v", parsedMedia)
	}

	nilMessage := &ClaudeMessage{}
	nilContent, err := nilMessage.ParseContent()
	if err != nil {
		t.Fatalf("expected nil content to parse without error, got %v", err)
	}
	if nilContent != nil {
		t.Fatalf("expected nil content, got %#v", nilContent)
	}

	stringMessage := &ClaudeMessage{Content: "plain-text"}
	if !stringMessage.IsStringContent() {
		t.Fatal("expected string content to stay string")
	}
	if got := stringMessage.GetStringContent(); got != "plain-text" {
		t.Fatalf("expected plain-text, got %q", got)
	}
}

func BenchmarkClaudeRequestSearchToolNameByToolCallId(b *testing.B) {
	req := &ClaudeRequest{
		Messages: make([]ClaudeMessage, 0, 64),
	}

	for i := 0; i < 64; i++ {
		req.Messages = append(req.Messages, ClaudeMessage{
			Role: "assistant",
			Content: []any{
				map[string]any{
					"type": "tool_use",
					"id":   "call_" + common.Interface2String(i),
					"name": "tool_" + common.Interface2String(i),
				},
				map[string]any{
					"type": "text",
					"text": "payload",
				},
			},
		})
	}

	b.Run("old_scan", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if got := oldSearchToolNameByToolCallID(req, "call_63"); got != "tool_63" {
				b.Fatalf("unexpected tool name %q", got)
			}
		}
	})

	b.Run("cached_index", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			req.toolNameByCallID = nil
			if got := req.SearchToolNameByToolCallId("call_63"); got != "tool_63" {
				b.Fatalf("unexpected tool name %q", got)
			}
		}
	})

	b.Run("cached_repeat_lookup", func(b *testing.B) {
		req.toolNameByCallID = nil
		if got := req.SearchToolNameByToolCallId("call_63"); got != "tool_63" {
			b.Fatalf("unexpected tool name %q", got)
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if got := req.SearchToolNameByToolCallId("call_63"); got != "tool_63" {
				b.Fatalf("unexpected tool name %q", got)
			}
		}
	})
}
