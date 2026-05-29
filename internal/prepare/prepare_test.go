package prepare

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestLineMode_OpenAI(t *testing.T) {
	input := "Hello world\nGoodbye world"
	var out bytes.Buffer

	err := Run(strings.NewReader(input), &out, Options{
		Provider: "openai",
		Model:    "gpt-4.1-nano",
		System:   "Be terse.",
	})
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}

	var req map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &req); err != nil {
		t.Fatal(err)
	}
	if req["custom_id"] != "line-1" {
		t.Errorf("expected custom_id=line-1, got %v", req["custom_id"])
	}
	if req["method"] != "POST" {
		t.Errorf("expected method=POST, got %v", req["method"])
	}

	body := req["body"].(map[string]any)
	if body["model"] != "gpt-4.1-nano" {
		t.Errorf("expected model=gpt-4.1-nano, got %v", body["model"])
	}
	msgs := body["messages"].([]any)
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages (system+user), got %d", len(msgs))
	}
}

func TestLineMode_Anthropic(t *testing.T) {
	input := "Hello world"
	var out bytes.Buffer

	err := Run(strings.NewReader(input), &out, Options{
		Provider:  "anthropic",
		Model:     "claude-haiku-4-5",
		MaxTokens: 512,
		System:    "Be helpful.",
	})
	if err != nil {
		t.Fatal(err)
	}

	var req map[string]any
	if err := json.Unmarshal(out.Bytes(), &req); err != nil {
		t.Fatal(err)
	}
	if req["custom_id"] != "line-1" {
		t.Errorf("expected custom_id=line-1, got %v", req["custom_id"])
	}

	params := req["params"].(map[string]any)
	if params["model"] != "claude-haiku-4-5" {
		t.Errorf("expected model=claude-haiku-4-5, got %v", params["model"])
	}
	if params["system"] != "Be helpful." {
		t.Errorf("expected system prompt, got %v", params["system"])
	}
	if params["max_tokens"] != float64(512) {
		t.Errorf("expected max_tokens=512, got %v", params["max_tokens"])
	}
}

func TestJSONLMode(t *testing.T) {
	input := `{"id": "item-1", "text": "Review this code", "lang": "python"}`
	var out bytes.Buffer

	err := Run(strings.NewReader(input), &out, Options{
		Provider: "openai",
		Model:    "gpt-4.1",
		Template: "Language: {{.lang}}\n{{.text}}",
	})
	if err != nil {
		t.Fatal(err)
	}

	var req map[string]any
	if err := json.Unmarshal(out.Bytes(), &req); err != nil {
		t.Fatal(err)
	}
	if req["custom_id"] != "item-1" {
		t.Errorf("expected custom_id=item-1, got %v", req["custom_id"])
	}

	body := req["body"].(map[string]any)
	msgs := body["messages"].([]any)
	userMsg := msgs[0].(map[string]any)
	content := userMsg["content"].(string)
	if !strings.Contains(content, "Language: python") {
		t.Errorf("template not applied: %s", content)
	}
}

func TestEmptyInput(t *testing.T) {
	var out bytes.Buffer
	err := Run(strings.NewReader(""), &out, Options{
		Provider: "openai",
		Model:    "gpt-4.1",
	})
	if err == nil {
		t.Fatal("expected error on empty input")
	}
}

func TestJSONMode_OpenAI(t *testing.T) {
	var out bytes.Buffer
	err := Run(strings.NewReader("List 3 colors"), &out, Options{
		Provider: "openai",
		Model:    "gpt-4.1",
		JSONMode: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	var req map[string]any
	if err := json.Unmarshal(out.Bytes(), &req); err != nil {
		t.Fatal(err)
	}
	body := req["body"].(map[string]any)
	rf := body["response_format"].(map[string]any)
	if rf["type"] != "json_object" {
		t.Errorf("expected json_object, got %v", rf["type"])
	}
}

func TestJSONSchema_OpenAI(t *testing.T) {
	schema := json.RawMessage(`{
		"title": "chess_puzzle",
		"type": "object",
		"properties": {
			"fen": {"type": "string"},
			"difficulty": {"type": "integer"}
		},
		"required": ["fen", "difficulty"],
		"additionalProperties": false
	}`)

	var out bytes.Buffer
	err := Run(strings.NewReader("Generate a chess puzzle"), &out, Options{
		Provider:   "openai",
		Model:      "gpt-4.1",
		JSONSchema: schema,
		JSONMode:   true,
	})
	if err != nil {
		t.Fatal(err)
	}

	var req map[string]any
	if err := json.Unmarshal(out.Bytes(), &req); err != nil {
		t.Fatal(err)
	}
	body := req["body"].(map[string]any)
	rf := body["response_format"].(map[string]any)
	if rf["type"] != "json_schema" {
		t.Fatalf("expected json_schema, got %v", rf["type"])
	}
	js := rf["json_schema"].(map[string]any)
	if js["name"] != "chess_puzzle" {
		t.Errorf("expected name=chess_puzzle, got %v", js["name"])
	}
	if js["strict"] != true {
		t.Errorf("expected strict=true")
	}
}

func TestJSONSchema_Anthropic(t *testing.T) {
	schema := json.RawMessage(`{
		"title": "chess_puzzle",
		"type": "object",
		"properties": {
			"fen": {"type": "string"},
			"difficulty": {"type": "integer"}
		},
		"required": ["fen", "difficulty"]
	}`)

	var out bytes.Buffer
	err := Run(strings.NewReader("Generate a chess puzzle"), &out, Options{
		Provider:   "anthropic",
		Model:      "claude-haiku-4-5",
		MaxTokens:  512,
		JSONSchema: schema,
		JSONMode:   true,
	})
	if err != nil {
		t.Fatal(err)
	}

	var req map[string]any
	if err := json.Unmarshal(out.Bytes(), &req); err != nil {
		t.Fatal(err)
	}
	params := req["params"].(map[string]any)

	tools := params["tools"].([]any)
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	tool := tools[0].(map[string]any)
	if tool["name"] != "chess_puzzle" {
		t.Errorf("expected tool name=chess_puzzle, got %v", tool["name"])
	}

	tc := params["tool_choice"].(map[string]any)
	if tc["type"] != "tool" || tc["name"] != "chess_puzzle" {
		t.Errorf("unexpected tool_choice: %v", tc)
	}
}

func parseOpenAIBody(t *testing.T, out []byte) map[string]any {
	t.Helper()
	var req map[string]any
	if err := json.Unmarshal(out, &req); err != nil {
		t.Fatalf("parsing output: %v", err)
	}
	return req["body"].(map[string]any)
}

func TestExtraBody_CLI(t *testing.T) {
	var out bytes.Buffer
	err := Run(strings.NewReader("hello"), &out, Options{
		Provider:  "openai",
		Model:     "gpt-4.1-nano",
		ExtraBody: map[string]any{"repetition_penalty": 1.05, "chat_template_kwargs": map[string]any{"enable_thinking": false}},
	})
	if err != nil {
		t.Fatal(err)
	}

	body := parseOpenAIBody(t, out.Bytes())
	if body["repetition_penalty"] != 1.05 {
		t.Errorf("expected repetition_penalty=1.05, got %v", body["repetition_penalty"])
	}
	ctk := body["chat_template_kwargs"].(map[string]any)
	if ctk["enable_thinking"] != false {
		t.Errorf("expected enable_thinking=false, got %v", ctk["enable_thinking"])
	}
}

func TestExtraBody_PerLineOverridesCLI(t *testing.T) {
	input := `{"text":"hello","extra_body":{"repetition_penalty":1.2},"temperature":0.3}`
	var out bytes.Buffer
	err := Run(strings.NewReader(input), &out, Options{
		Provider:  "openai",
		Model:     "gpt-4.1-nano",
		ExtraBody: map[string]any{"repetition_penalty": 1.05},
	})
	if err != nil {
		t.Fatal(err)
	}

	body := parseOpenAIBody(t, out.Bytes())
	if body["repetition_penalty"] != 1.2 {
		t.Errorf("per-line extra_body should override CLI: expected 1.2, got %v", body["repetition_penalty"])
	}
	if body["temperature"] != 0.3 {
		t.Errorf("per-line temperature should apply: expected 0.3, got %v", body["temperature"])
	}
}

func TestExtraBody_DeepMerge(t *testing.T) {
	input := `{"text":"hello","extra_body":{"chat_template_kwargs":{"thinking_budget":200}}}`
	var out bytes.Buffer
	err := Run(strings.NewReader(input), &out, Options{
		Provider:  "openai",
		Model:     "gpt-4.1-nano",
		ExtraBody: map[string]any{"chat_template_kwargs": map[string]any{"enable_thinking": true}},
	})
	if err != nil {
		t.Fatal(err)
	}

	body := parseOpenAIBody(t, out.Bytes())
	ctk := body["chat_template_kwargs"].(map[string]any)
	if ctk["enable_thinking"] != true {
		t.Errorf("deep merge should preserve CLI enable_thinking=true, got %v", ctk["enable_thinking"])
	}
	if ctk["thinking_budget"] != float64(200) {
		t.Errorf("deep merge should add per-line thinking_budget=200, got %v", ctk["thinking_budget"])
	}
}

func TestTemperature_CLI(t *testing.T) {
	temp := 0.7
	var out bytes.Buffer
	err := Run(strings.NewReader("hello"), &out, Options{
		Provider:    "openai",
		Model:       "gpt-4.1-nano",
		Temperature: &temp,
	})
	if err != nil {
		t.Fatal(err)
	}

	body := parseOpenAIBody(t, out.Bytes())
	if body["temperature"] != 0.7 {
		t.Errorf("expected temperature=0.7, got %v", body["temperature"])
	}
}

func TestReservedKeys_StrippedFromTemplate(t *testing.T) {
	input := `{"text":"hello","temperature":0.3,"extra_body":{"foo":1}}`
	var out bytes.Buffer
	err := Run(strings.NewReader(input), &out, Options{
		Provider: "openai",
		Model:    "gpt-4.1-nano",
		Template: "content={{.text}} temp={{.temperature}}",
	})
	if err != nil {
		t.Fatal(err)
	}

	body := parseOpenAIBody(t, out.Bytes())
	msgs := body["messages"].([]any)
	content := msgs[0].(map[string]any)["content"].(string)

	if strings.Contains(content, "temp=0.3") {
		t.Error("reserved key 'temperature' should not be available in template")
	}
	if !strings.Contains(content, "content=hello") {
		t.Error("non-reserved key 'text' should be available in template")
	}
}

func TestJSONMode_Anthropic_AddsSystemHint(t *testing.T) {
	var out bytes.Buffer
	err := Run(strings.NewReader("List colors"), &out, Options{
		Provider:  "anthropic",
		Model:     "claude-haiku-4-5",
		MaxTokens: 256,
		JSONMode:  true,
	})
	if err != nil {
		t.Fatal(err)
	}

	var req map[string]any
	if err := json.Unmarshal(out.Bytes(), &req); err != nil {
		t.Fatal(err)
	}
	params := req["params"].(map[string]any)
	sys, ok := params["system"].(string)
	if !ok || !strings.Contains(sys, "JSON") {
		t.Errorf("expected system prompt with JSON hint, got %v", params["system"])
	}
}
