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

func writeChatResponse(w http.ResponseWriter, model string) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":    "chatcmpl-1",
		"model": model,
		"choices": []map[string]any{{
			"index":         0,
			"message":       map[string]string{"role": "assistant", "content": "hello"},
			"finish_reason": "stop",
		}},
		"usage": map[string]int{"prompt_tokens": 5, "completion_tokens": 3, "total_tokens": 8},
	})
}

func writeInputFile(t *testing.T, path string, row map[string]any) {
	t.Helper()
	data, _ := json.Marshal(row)
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		t.Fatalf("writing input: %v", err)
	}
}

func TestDirectRunner_NoAuthHeader(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		writeChatResponse(w, "/models/Qwen3.5-122B")
	}))
	defer srv.Close()

	runner := NewDirectRunner("", srv.URL, "openai", 1)

	input := filepath.Join(t.TempDir(), "input.jsonl")
	output := filepath.Join(t.TempDir(), "output.jsonl")

	writeInputFile(t, input, map[string]any{
		"custom_id": "test-1",
		"method":    "POST",
		"url":       "/v1/chat/completions",
		"body": map[string]any{
			"model":    "/models/Qwen3.5-122B",
			"messages": []map[string]string{{"role": "user", "content": "hi"}},
		},
	})

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
		if err := json.Unmarshal(body, &req); err == nil {
			gotModel, _ = req["model"].(string)
		}
		writeChatResponse(w, gotModel)
	}))
	defer srv.Close()

	runner := NewDirectRunner("", srv.URL, "openai", 1)

	input := filepath.Join(t.TempDir(), "input.jsonl")
	output := filepath.Join(t.TempDir(), "output.jsonl")

	writeInputFile(t, input, map[string]any{
		"custom_id": "slash-model",
		"method":    "POST",
		"url":       "/v1/chat/completions",
		"body": map[string]any{
			"model":    "/models/Qwen3.5-122B-A10B-NVFP4",
			"messages": []map[string]string{{"role": "user", "content": "test"}},
		},
	})

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
		writeChatResponse(w, "test-model")
	}))
	defer srv.Close()

	runner := NewDirectRunner("sk-test", srv.URL, "openai", 1)

	input := filepath.Join(t.TempDir(), "input.jsonl")
	output := filepath.Join(t.TempDir(), "output.jsonl")

	writeInputFile(t, input, map[string]any{
		"custom_id": "lat-1",
		"method":    "POST",
		"url":       "/v1/chat/completions",
		"body": map[string]any{
			"model":    "test-model",
			"messages": []map[string]string{{"role": "user", "content": "hi"}},
		},
	})

	if _, err := runner.Run(t.Context(), input, output, RunOpts{}); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	outData, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("reading output: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal(outData, &result); err != nil {
		t.Fatalf("parsing output: %v", err)
	}

	latency, ok := result["latency_ms"]
	if !ok {
		t.Fatal("expected latency_ms field in output")
	}
	if lat, ok := latency.(float64); !ok || lat < 0 {
		t.Errorf("expected latency_ms >= 0, got %v", latency)
	}
}
