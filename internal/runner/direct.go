package runner

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/prototypeasap/cheapshot/internal/jsonl"
	"github.com/prototypeasap/cheapshot/internal/provider"
)

type DirectRunner struct {
	apiKey      string
	baseURL     string
	format      string
	concurrency int
	client      *http.Client
	retry       provider.RetryConfig
	loggedURL   sync.Once
}

func NewDirectRunner(apiKey, baseURL, format string, concurrency int, timeout time.Duration) *DirectRunner {
	if timeout == 0 {
		timeout = 5 * time.Minute
	}
	return &DirectRunner{
		apiKey:      apiKey,
		baseURL:     strings.TrimRight(baseURL, "/"),
		format:      format,
		concurrency: concurrency,
		client:      &http.Client{Timeout: timeout},
		retry:       provider.DefaultRetryConfig(),
	}
}

type workItem struct {
	customID string
	body     json.RawMessage
	lineNum  int
}

type resultItem struct {
	lineNum int
	data    []byte
	failed  bool
}

func (r *DirectRunner) Run(ctx context.Context, inputPath, outputPath string, _ RunOpts) (*RunResult, error) {
	items, err := r.readInput(inputPath)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("empty input")
	}

	fmt.Fprintf(os.Stderr, "Processing %d requests (concurrency=%d, format=%s, timeout=%s)...\n",
		len(items), r.concurrency, r.format, r.client.Timeout)

	results := make([]resultItem, len(items))
	var succeeded, failed atomic.Int64
	var lastComplete atomic.Int64
	lastComplete.Store(time.Now().UnixMilli())

	work := make(chan int, len(items))
	for i := range items {
		work <- i
	}
	close(work)

	ticker := time.NewTicker(2 * time.Second)
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				s, f := succeeded.Load(), failed.Load()
				total := s + f
				elapsed := time.Since(time.UnixMilli(lastComplete.Load())).Truncate(time.Second)
				if total < int64(len(items)) {
					fmt.Fprintf(os.Stderr, "\rProcessing... %d/%d completed (%d failed) [%s since last]", total, len(items), f, elapsed)
				}
			case <-done:
				return
			}
		}
	}()

	var wg sync.WaitGroup
	for range min(r.concurrency, len(items)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range work {
				if ctx.Err() != nil {
					return
				}
				item := items[idx]
				data, isFailed := r.executeOne(ctx, item)
				results[idx] = resultItem{lineNum: item.lineNum, data: data, failed: isFailed}
				if isFailed {
					failed.Add(1)
				} else {
					succeeded.Add(1)
				}
				lastComplete.Store(time.Now().UnixMilli())
				total := succeeded.Load() + failed.Load()
				fmt.Fprintf(os.Stderr, "\rProcessing... %d/%d completed (%d failed)                    ", total, len(items), failed.Load())
			}
		}()
	}

	wg.Wait()
	ticker.Stop()
	close(done)
	fmt.Fprintln(os.Stderr)

	if err := r.writeOutput(outputPath, results); err != nil {
		return nil, err
	}

	s, f := int(succeeded.Load()), int(failed.Load())
	fmt.Fprintf(os.Stderr, "Done. %d succeeded, %d failed. Results: %s\n", s, f, outputPath)

	return &RunResult{Succeeded: s, Failed: f, Total: s + f}, nil
}

func (r *DirectRunner) readInput(path string) ([]workItem, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var items []workItem
	scanner := jsonl.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		lineNum++

		var parsed struct {
			CustomID string          `json:"custom_id"`
			Body     json.RawMessage `json:"body"`
			Params   json.RawMessage `json:"params"`
		}
		if err := json.Unmarshal(line, &parsed); err != nil {
			return nil, fmt.Errorf("line %d: invalid JSON: %w", lineNum, err)
		}

		body := parsed.Body
		if r.format == "anthropic" {
			body = parsed.Params
		}

		items = append(items, workItem{
			customID: parsed.CustomID,
			body:     body,
			lineNum:  lineNum,
		})
	}
	return items, scanner.Err()
}

func (r *DirectRunner) executeOne(ctx context.Context, item workItem) ([]byte, bool) {
	endpoint := r.chatEndpoint()
	r.loggedURL.Do(func() { fmt.Fprintf(os.Stderr, "→ POST %s\n", endpoint) })

	start := time.Now()
	resp, err := provider.DoWithRetry(ctx, r.retry, func() (*http.Response, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(item.body))
		if err != nil {
			return nil, err
		}
		r.setHeaders(req)
		return r.client.Do(req)
	})
	latencyMs := time.Since(start).Milliseconds()

	if err != nil {
		return r.wrapError(item.customID, fmt.Sprintf("request failed: %v", err)), true
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return r.wrapError(item.customID, fmt.Sprintf("reading response: %v", err)), true
	}

	if resp.StatusCode >= 400 {
		return r.wrapError(item.customID, fmt.Sprintf("HTTP %d: %s", resp.StatusCode, truncate(respBody, 512))), true
	}

	return r.wrapSuccess(item.customID, resp.StatusCode, respBody, latencyMs), false
}

func (r *DirectRunner) chatEndpoint() string {
	if r.format == "anthropic" {
		return r.baseURL + "/v1/messages"
	}
	return r.baseURL + "/v1/chat/completions"
}

func (r *DirectRunner) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	if r.apiKey == "" {
		return
	}
	if r.format == "anthropic" {
		req.Header.Set("x-api-key", r.apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")
	} else {
		req.Header.Set("Authorization", "Bearer "+r.apiKey)
	}
}

func (r *DirectRunner) wrapSuccess(customID string, statusCode int, body json.RawMessage, latencyMs int64) []byte {
	var out []byte
	if r.format == "anthropic" {
		out, _ = json.Marshal(map[string]any{
			"custom_id":  customID,
			"latency_ms": latencyMs,
			"result": map[string]any{
				"type":    "succeeded",
				"message": body,
			},
		})
	} else {
		out, _ = json.Marshal(map[string]any{
			"id":         customID,
			"custom_id":  customID,
			"latency_ms": latencyMs,
			"response": map[string]any{
				"status_code": statusCode,
				"body":        body,
			},
		})
	}
	return out
}

func (r *DirectRunner) wrapError(customID, msg string) []byte {
	var out []byte
	if r.format == "anthropic" {
		out, _ = json.Marshal(map[string]any{
			"custom_id": customID,
			"result": map[string]any{
				"type":  "errored",
				"error": map[string]string{"type": "api_error", "message": msg},
			},
		})
	} else {
		out, _ = json.Marshal(map[string]any{
			"id":        customID,
			"custom_id": customID,
			"response": map[string]any{
				"status_code": 500,
				"body":        nil,
			},
			"error": map[string]string{"code": "request_failed", "message": msg},
		})
	}
	return out
}

func (r *DirectRunner) writeOutput(path string, results []resultItem) error {
	tmpPath := path + ".tmp"
	out, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	defer out.Close() //nolint:errcheck // closed explicitly before rename
	bw := bufio.NewWriter(out)

	for _, res := range results {
		if res.data == nil {
			continue
		}
		if _, err := bw.Write(res.data); err != nil {
			_ = os.Remove(tmpPath)
			return err
		}
		if _, err := bw.WriteString("\n"); err != nil {
			_ = os.Remove(tmpPath)
			return err
		}
	}
	if err := bw.Flush(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}

	_ = out.Close()
	return os.Rename(tmpPath, path)
}

func truncate(b []byte, maxLen int) string {
	if len(b) <= maxLen {
		return string(b)
	}
	return string(b[:maxLen]) + "..."
}
