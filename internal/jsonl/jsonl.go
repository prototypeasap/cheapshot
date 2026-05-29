package jsonl

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

func NewScanner(r io.Reader) *bufio.Scanner {
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)
	return s
}

func NewScannerFromBytes(data []byte) *bufio.Scanner {
	return NewScanner(bytes.NewReader(data))
}

func CountLines(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	count := 0
	scanner := NewScanner(f)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) > 0 {
			count++
		}
	}
	return count, scanner.Err()
}

func ValidateOpenAI(input []byte) error {
	scanner := NewScannerFromBytes(input)
	lineNum := 0
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		lineNum++
		var obj struct {
			CustomID string          `json:"custom_id"`
			Method   string          `json:"method"`
			URL      string          `json:"url"`
			Body     json.RawMessage `json:"body"`
		}
		if err := json.Unmarshal(line, &obj); err != nil {
			return fmt.Errorf("line %d: invalid JSON: %w", lineNum, err)
		}
		if obj.CustomID == "" {
			return fmt.Errorf("line %d: missing custom_id", lineNum)
		}
		if obj.Body == nil {
			return fmt.Errorf("line %d: missing body", lineNum)
		}
	}
	if lineNum == 0 {
		return fmt.Errorf("input file is empty")
	}
	return scanner.Err()
}

func ValidateAnthropic(input []byte) error {
	scanner := NewScannerFromBytes(input)
	lineNum := 0
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		lineNum++
		var obj struct {
			CustomID string          `json:"custom_id"`
			Params   json.RawMessage `json:"params"`
		}
		if err := json.Unmarshal(line, &obj); err != nil {
			return fmt.Errorf("line %d: invalid JSON: %w", lineNum, err)
		}
		if obj.CustomID == "" {
			return fmt.Errorf("line %d: missing custom_id", lineNum)
		}
		if obj.Params == nil {
			return fmt.Errorf("line %d: missing params", lineNum)
		}
	}
	if lineNum == 0 {
		return fmt.Errorf("input file is empty")
	}
	return scanner.Err()
}
