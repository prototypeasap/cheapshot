package cli

import (
	"encoding/json"
	"testing"
)

func TestExtractMeta_OpenAI(t *testing.T) {
	line := []byte(`{
		"custom_id": "line-1",
		"latency_ms": 234,
		"response": {
			"status_code": 200,
			"body": {
				"model": "gpt-4.1-nano",
				"choices": [{"index": 0, "message": {"role": "assistant", "content": "Paris"}, "finish_reason": "stop"}],
				"usage": {"prompt_tokens": 15, "completion_tokens": 3, "total_tokens": 18}
			}
		}
	}`)

	meta := extractMeta(line)

	if meta["model"] != "gpt-4.1-nano" {
		t.Errorf("expected model=gpt-4.1-nano, got %v", meta["model"])
	}
	if meta["input_tokens"] != 15 {
		t.Errorf("expected input_tokens=15, got %v", meta["input_tokens"])
	}
	if meta["output_tokens"] != 3 {
		t.Errorf("expected output_tokens=3, got %v", meta["output_tokens"])
	}
	if meta["finish_reason"] != "stop" {
		t.Errorf("expected finish_reason=stop, got %v", meta["finish_reason"])
	}
	latency, ok := meta["latency_ms"].(json.Number)
	if !ok || latency.String() != "234" {
		t.Errorf("expected latency_ms=234, got %v", meta["latency_ms"])
	}
}

func TestExtractMeta_Anthropic(t *testing.T) {
	line := []byte(`{
		"custom_id": "line-1",
		"result": {
			"type": "succeeded",
			"message": {
				"model": "claude-haiku-4-5-20251001",
				"stop_reason": "end_turn",
				"content": [{"type": "text", "text": "hello"}],
				"usage": {
					"input_tokens": 100,
					"output_tokens": 25,
					"cache_read_input_tokens": 50,
					"cache_creation_input_tokens": 10
				}
			}
		}
	}`)

	meta := extractMeta(line)

	if meta["model"] != "claude-haiku-4-5-20251001" {
		t.Errorf("expected model=claude-haiku-4-5-20251001, got %v", meta["model"])
	}
	if meta["input_tokens"] != 100 {
		t.Errorf("expected input_tokens=100, got %v", meta["input_tokens"])
	}
	if meta["output_tokens"] != 25 {
		t.Errorf("expected output_tokens=25, got %v", meta["output_tokens"])
	}
	if meta["cache_read_tokens"] != 50 {
		t.Errorf("expected cache_read_tokens=50, got %v", meta["cache_read_tokens"])
	}
	if meta["cache_write_tokens"] != 10 {
		t.Errorf("expected cache_write_tokens=10, got %v", meta["cache_write_tokens"])
	}
	if meta["finish_reason"] != "end_turn" {
		t.Errorf("expected finish_reason=end_turn, got %v", meta["finish_reason"])
	}
}

func TestStripThinking(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple thinking block",
			input: "<think>\nLet me reason about this...\n</think>\n\nThe answer is 42.",
			want:  "The answer is 42.",
		},
		{
			name:  "no thinking block",
			input: "Just a normal response.",
			want:  "Just a normal response.",
		},
		{
			name:  "thinking with JSON response",
			input: "<think>\nI need to produce JSON.\n</think>\n\n{\"answer\": 42}",
			want:  "{\"answer\": 42}",
		},
		{
			name:  "empty thinking block",
			input: "<think></think>\n\nResult here.",
			want:  "Result here.",
		},
		{
			name:  "unclosed thinking block",
			input: "<think>\nModel ran out of tokens mid-thought",
			want:  "<think>\nModel ran out of tokens mid-thought",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripThinkingBlocks(tt.input)
			if got != tt.want {
				t.Errorf("stripThinkingBlocks(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestExtractMeta_NoLatency(t *testing.T) {
	line := []byte(`{
		"custom_id": "batch-1",
		"response": {
			"status_code": 200,
			"body": {
				"model": "gpt-4.1-nano",
				"choices": [{"index": 0, "message": {"role": "assistant", "content": "ok"}, "finish_reason": "stop"}],
				"usage": {"prompt_tokens": 10, "completion_tokens": 1, "total_tokens": 11}
			}
		}
	}`)

	meta := extractMeta(line)

	if _, ok := meta["latency_ms"]; ok {
		t.Error("batch results should not have latency_ms")
	}
	if meta["model"] != "gpt-4.1-nano" {
		t.Errorf("expected model, got %v", meta["model"])
	}
}
