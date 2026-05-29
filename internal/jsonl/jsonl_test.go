package jsonl

import (
	"testing"
)

func TestValidateOpenAI_Valid(t *testing.T) {
	input := []byte(`{"custom_id": "req-1", "method": "POST", "url": "/v1/chat/completions", "body": {"model": "gpt-4.1"}}
{"custom_id": "req-2", "method": "POST", "url": "/v1/chat/completions", "body": {"model": "gpt-4.1"}}`)

	if err := ValidateOpenAI(input); err != nil {
		t.Fatalf("expected valid, got: %v", err)
	}
}

func TestValidateOpenAI_MissingCustomID(t *testing.T) {
	input := []byte(`{"method": "POST", "url": "/v1/chat/completions", "body": {"model": "gpt-4.1"}}`)
	err := ValidateOpenAI(input)
	if err == nil {
		t.Fatal("expected error for missing custom_id")
	}
}

func TestValidateOpenAI_MissingBody(t *testing.T) {
	input := []byte(`{"custom_id": "req-1", "method": "POST", "url": "/v1/chat/completions"}`)
	err := ValidateOpenAI(input)
	if err == nil {
		t.Fatal("expected error for missing body")
	}
}

func TestValidateOpenAI_Empty(t *testing.T) {
	err := ValidateOpenAI([]byte(""))
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestValidateAnthropic_Valid(t *testing.T) {
	input := []byte(`{"custom_id": "req-1", "params": {"model": "claude-haiku-4-5", "max_tokens": 1024, "messages": []}}`)
	if err := ValidateAnthropic(input); err != nil {
		t.Fatalf("expected valid, got: %v", err)
	}
}

func TestValidateAnthropic_MissingParams(t *testing.T) {
	input := []byte(`{"custom_id": "req-1"}`)
	err := ValidateAnthropic(input)
	if err == nil {
		t.Fatal("expected error for missing params")
	}
}
