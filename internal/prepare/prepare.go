package prepare

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Options struct {
	Provider    string
	Model       string
	MaxTokens   int
	System      string
	Template    string
	FileInput   bool
	JSONMode    bool
	JSONSchema  json.RawMessage
	ExtraBody   map[string]any
	Temperature *float64
}

var reservedKeys = map[string]bool{
	"extra_body": true, "temperature": true, "top_p": true, "top_k": true,
	"seed": true, "stop": true, "max_tokens": true, "presence_penalty": true,
	"frequency_penalty": true, "min_p": true,
}

func DeepMerge(dst, src map[string]any) {
	for k, v := range src {
		if srcMap, ok := v.(map[string]any); ok {
			if dstMap, ok := dst[k].(map[string]any); ok {
				DeepMerge(dstMap, srcMap)
				continue
			}
		}
		dst[k] = v
	}
}

func Run(input io.Reader, output io.Writer, opts Options) error { //nolint:gocritic // opts passed by value intentionally for public API stability
	var parsedSchema *schemaInfo
	if opts.JSONSchema != nil {
		var schema map[string]any
		if err := json.Unmarshal(opts.JSONSchema, &schema); err != nil {
			return fmt.Errorf("invalid JSON schema: %w", err)
		}
		name, _ := schema["title"].(string)
		parsedSchema = &schemaInfo{name: name, schema: schema}
	}

	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

	lineNum := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		lineNum++

		content, customID, perLine, err := parseLine(line, lineNum, &opts)
		if err != nil {
			return err
		}

		jsonLine, err := formatLine(customID, content, &opts, parsedSchema, perLine)
		if err != nil {
			return fmt.Errorf("line %d: %w", lineNum, err)
		}

		if _, err = output.Write(jsonLine); err != nil {
			return fmt.Errorf("line %d: writing output: %w", lineNum, err)
		}
		if _, err = output.Write([]byte("\n")); err != nil {
			return fmt.Errorf("line %d: writing output: %w", lineNum, err)
		}
	}

	if lineNum == 0 {
		return fmt.Errorf("empty input")
	}
	return scanner.Err()
}

type schemaInfo struct {
	name   string
	schema map[string]any
}

func formatOpenAI(customID, content string, opts *Options, si *schemaInfo, perLine map[string]any) ([]byte, error) {
	messages := []map[string]string{}
	if opts.System != "" {
		messages = append(messages, map[string]string{"role": "system", "content": opts.System})
	}
	messages = append(messages, map[string]string{"role": "user", "content": content})

	body := map[string]any{
		"model":    opts.Model,
		"messages": messages,
	}
	if opts.MaxTokens > 0 {
		body["max_tokens"] = opts.MaxTokens
	}

	if si != nil {
		name := si.name
		if name == "" {
			name = "response"
		}
		body["response_format"] = map[string]any{
			"type": "json_schema",
			"json_schema": map[string]any{
				"name":   name,
				"strict": true,
				"schema": si.schema,
			},
		}
	} else if opts.JSONMode {
		body["response_format"] = map[string]any{"type": "json_object"}
	}

	if opts.Temperature != nil {
		body["temperature"] = *opts.Temperature
	}
	if opts.ExtraBody != nil {
		DeepMerge(body, opts.ExtraBody)
	}
	applyPerLine(body, perLine)

	req := map[string]any{
		"custom_id": customID,
		"method":    "POST",
		"url":       "/v1/chat/completions",
		"body":      body,
	}
	return json.Marshal(req)
}

func formatAnthropic(customID, content string, opts *Options, si *schemaInfo, perLine map[string]any) ([]byte, error) {
	params := map[string]any{
		"model": opts.Model,
		"messages": []map[string]string{
			{"role": "user", "content": content},
		},
	}
	if opts.MaxTokens > 0 {
		params["max_tokens"] = opts.MaxTokens
	} else {
		params["max_tokens"] = 1024
	}

	sys := opts.System
	if si != nil {
		name := si.name
		if name == "" {
			name = "structured_output"
		}
		params["tools"] = []map[string]any{{
			"name":         name,
			"description":  "Record the structured response",
			"input_schema": si.schema,
		}}
		params["tool_choice"] = map[string]string{"type": "tool", "name": name}
	} else if opts.JSONMode {
		if sys == "" {
			sys = "Respond with valid JSON only."
		} else if !strings.Contains(strings.ToLower(sys), "json") {
			sys += "\n\nRespond with valid JSON only."
		}
	}

	if sys != "" {
		params["system"] = sys
	}

	if opts.Temperature != nil {
		params["temperature"] = *opts.Temperature
	}
	if opts.ExtraBody != nil {
		DeepMerge(params, opts.ExtraBody)
	}
	applyPerLine(params, perLine)

	req := map[string]any{
		"custom_id": customID,
		"params":    params,
	}
	return json.Marshal(req)
}

func parseLine(line string, lineNum int, opts *Options) (content, customID string, perLine map[string]any, err error) {
	switch {
	case opts.FileInput:
		path := line
		data, err := os.ReadFile(path)
		if err != nil {
			return "", "", nil, fmt.Errorf("line %d: reading file %s: %w", lineNum, path, err)
		}
		content = applyTemplate(opts.Template, map[string]string{
			"Name":    filepath.Base(path),
			"Path":    path,
			"Content": string(data),
		})
		return content, sanitizeID(path), nil, nil
	case isJSON(line):
		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			return "", "", nil, fmt.Errorf("line %d: invalid JSON: %w", lineNum, err)
		}
		fields := make(map[string]string)
		for k, v := range obj {
			if !reservedKeys[k] {
				fields[k] = fmt.Sprintf("%v", v)
			}
		}
		content = applyTemplate(opts.Template, fields)
		customID = fmt.Sprintf("line-%d", lineNum)
		if id, ok := obj["id"]; ok {
			customID = sanitizeID(fmt.Sprintf("%v", id))
		}
		return content, customID, extractPerLine(obj), nil
	default:
		if opts.Template != "" {
			content = applyTemplate(opts.Template, map[string]string{
				"text":    line,
				"Content": line,
			})
		} else {
			content = line
		}
		return content, fmt.Sprintf("line-%d", lineNum), nil, nil
	}
}

func formatLine(customID, content string, opts *Options, si *schemaInfo, perLine map[string]any) ([]byte, error) {
	switch opts.Provider {
	case "openai":
		return formatOpenAI(customID, content, opts, si, perLine)
	case "anthropic":
		return formatAnthropic(customID, content, opts, si, perLine)
	default:
		return nil, fmt.Errorf("unknown provider: %s", opts.Provider)
	}
}

func extractPerLine(obj map[string]any) map[string]any {
	pl := make(map[string]any)
	for k, v := range obj {
		if reservedKeys[k] {
			pl[k] = v
		}
	}
	if len(pl) == 0 {
		return nil
	}
	return pl
}

func applyPerLine(body, perLine map[string]any) {
	if perLine == nil {
		return
	}
	for k, v := range perLine {
		if k == "extra_body" {
			if eb, ok := v.(map[string]any); ok {
				DeepMerge(body, eb)
			}
			continue
		}
		body[k] = v
	}
}

func applyTemplate(tmpl string, fields map[string]string) string {
	if tmpl == "" {
		if v, ok := fields["Content"]; ok {
			return v
		}
		if v, ok := fields["text"]; ok {
			return v
		}
		for _, v := range fields {
			return v
		}
		return ""
	}

	result := tmpl
	for k, v := range fields {
		result = strings.ReplaceAll(result, "{{."+k+"}}", v)
	}
	result = strings.ReplaceAll(result, `\n`, "\n")
	return result
}

func sanitizeID(s string) string {
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, "\\", "-")
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, ".", "-")
	if len(s) > 64 {
		s = s[len(s)-64:]
	}
	return s
}

func isJSON(s string) bool {
	return s != "" && s[0] == '{'
}
