package runner

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDirectRunner_NoAuthHeader(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id":    "chatcmpl-1",
			"model": "/models/Qwen3.5-122B",
			"choices": []map[string]any{{
				"index":         0,
				"message":       map[string]string{"role": "assistant", "content": "hello"},
				"finish_reason": "stop",
			}},
			"usage": map[string]int{"prompt_tokens": 5, "completion_tokens": 3, "total_tokens": 8},
		})
	}))
	defer srv.Close()

	runner := NewDirectRunner("", srv.URL, "openai", 1)

	input := filepath.Join(t.TempDir(), "input.jsonl")
	output := filepath.Join(t.TempDir(), "output.jsonl")

	body := map[string]any{
		"model":    "/models/Qwen3.5-122B",
		"messages": []map[string]string{{"role": "user", "content": "hi"}},
	}
	row := map[string]any{
		"custom_id": "test-1",
		"method":    "POST",
		"url":       "/v1/chat/completions",
		"body":      body,
	}
	data, _ := json.Marshal(row)
	os.WriteFile(input, append(data, '\n'), 0o644)

	result, err := runner.Run(t.Context(), input, output, RunOpts{})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if result.Succeeded != 1 {
		t.Fatalf("expected 1 succeeded, got %d", result.Succeeded)
	}
	if gotAuth != "" {
		t.Errorf("expected no Authorization header, got %q", gotAuth)
	}
}

func TestDirectRunner_SlashModelName(t *testing.T) {
	var gotModel string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		json.Unmarshal(body, &req)
		gotModel, _ = req["model"].(string)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id":    "chatcmpl-1",
			"model": gotModel,
			"choices": []map[string]any{{
				"index":         0,
				"message":       map[string]string{"role": "assistant", "content": "ok"},
				"finish_reason": "stop",
			}},
			"usage": map[string]int{"prompt_tokens": 5, "completion_tokens": 1, "total_tokens": 6},
		})
	}))
	defer srv.Close()

	runner := NewDirectRunner("", srv.URL, "openai", 1)

	input := filepath.Join(t.TempDir(), "input.jsonl")
	output := filepath.Join(t.TempDir(), "output.jsonl")

	body := map[string]any{
		"model":    "/models/Qwen3.5-122B-A10B-NVFP4",
		"messages": []map[string]string{{"role": "user", "content": "test"}},
	}
	row := map[string]any{
		"custom_id": "slash-model",
		"method":    "POST",
		"url":       "/v1/chat/completions",
		"body":      body,
	}
	data, _ := json.Marshal(row)
	os.WriteFile(input, append(data, '\n'), 0o644)

	result, err := runner.Run(t.Context(), input, output, RunOpts{})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if result.Succeeded != 1 {
		t.Fatalf("expected 1 succeeded, got %d", result.Succeeded)
	}
	if gotModel != "/models/Qwen3.5-122B-A10B-NVFP4" {
		t.Errorf("expected model /models/Qwen3.5-122B-A10B-NVFP4, got %q", gotModel)
	}
}

func TestDirectRunner_LatencyMs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id":    "chatcmpl-1",
			"model": "test-model",
			"choices": []map[string]any{{
				"index":         0,
				"message":       map[string]string{"role": "assistant", "content": "hi"},
				"finish_reason": "stop",
			}},
			"usage": map[string]int{"prompt_tokens": 5, "completion_tokens": 1, "total_tokens": 6},
		})
	}))
	defer srv.Close()

	runner := NewDirectRunner("sk-test", srv.URL, "openai", 1)

	input := filepath.Join(t.TempDir(), "input.jsonl")
	output := filepath.Join(t.TempDir(), "output.jsonl")

	body := map[string]any{
		"model":    "test-model",
		"messages": []map[string]string{{"role": "user", "content": "hi"}},
	}
	row := map[string]any{"custom_id": "lat-1", "method": "POST", "url": "/v1/chat/completions", "body": body}
	data, _ := json.Marshal(row)
	os.WriteFile(input, append(data, '\n'), 0o644)

	_, err := runner.Run(t.Context(), input, output, RunOpts{})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	outData, _ := os.ReadFile(output)
	var result map[string]any
	json.Unmarshal(outData, &result)

	latency, ok := result["latency_ms"]
	if !ok {
		t.Fatal("expected latency_ms field in output")
	}
	if lat, ok := latency.(float64); !ok || lat < 0 {
		t.Errorf("expected latency_ms >= 0, got %v", latency)
	}
}
