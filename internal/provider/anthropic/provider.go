package anthropic

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
	"time"

	"github.com/prototypeasap/cheapshot/internal/jsonl"
	"github.com/prototypeasap/cheapshot/internal/provider"
)

type Provider struct {
	apiKey     string
	baseURL    string
	apiVersion string
	client     *http.Client
	retry      provider.RetryConfig
}

func New(apiKey, baseURL string) *Provider {
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	return &Provider{
		apiKey:     apiKey,
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiVersion: "2023-06-01",
		client:     &http.Client{Timeout: 5 * time.Minute},
		retry:      provider.DefaultRetryConfig(),
	}
}

func (p *Provider) Name() string { return "anthropic" }

func (p *Provider) Validate(input []byte) error {
	return jsonl.ValidateAnthropic(input)
}

func (p *Provider) Submit(ctx context.Context, inputPath string, _ provider.SubmitOpts) (*provider.SubmitResult, error) {
	requests, count, err := p.readRequests(inputPath)
	if err != nil {
		return nil, fmt.Errorf("reading input: %w", err)
	}

	body, err := json.Marshal(map[string]any{
		"requests": requests,
	})
	if err != nil {
		return nil, fmt.Errorf("marshaling requests: %w", err)
	}

	resp, err := provider.DoWithRetry(ctx, p.retry, func() (*http.Response, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/messages/batches", bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		p.setHeaders(req)
		return p.client.Do(req)
	})
	if err != nil {
		return nil, fmt.Errorf("creating batch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("create batch failed (HTTP %d): %s", resp.StatusCode, respBody)
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &provider.SubmitResult{
		RemoteBatchID: result.ID,
		RequestCount:  count,
	}, nil
}

func (p *Provider) Status(ctx context.Context, batchID string) (*provider.BatchStatus, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/v1/messages/batches/"+batchID, http.NoBody)
	if err != nil {
		return nil, err
	}
	p.setHeaders(req)

	resp, err := provider.DoWithRetry(ctx, p.retry, func() (*http.Response, error) {
		return p.client.Do(req)
	})
	if err != nil {
		return nil, fmt.Errorf("polling batch: %w", err)
	}
	defer resp.Body.Close()

	var batch batchResponse
	if err := json.NewDecoder(resp.Body).Decode(&batch); err != nil {
		return nil, fmt.Errorf("decoding batch status: %w", err)
	}

	terminal := batch.ProcessingStatus == "ended"

	return &provider.BatchStatus{
		Raw:       batch.ProcessingStatus,
		Terminal:  terminal,
		Succeeded: batch.RequestCounts.Succeeded,
		Failed:    batch.RequestCounts.Errored,
		Expired:   batch.RequestCounts.Expired,
		Canceled:  batch.RequestCounts.Canceled,
		Total:     batch.RequestCounts.Processing + batch.RequestCounts.Succeeded + batch.RequestCounts.Errored + batch.RequestCounts.Expired + batch.RequestCounts.Canceled,
		ResultURL: batch.ResultsURL,
	}, nil
}

func (p *Provider) Results(ctx context.Context, batchID, outputPath string) (*provider.ResultSummary, error) {
	status, err := p.Status(ctx, batchID)
	if err != nil {
		return nil, err
	}
	if !status.Terminal {
		return nil, fmt.Errorf("batch %s is not terminal (status: %s)", batchID, status.Raw)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/v1/messages/batches/"+batchID+"/results", http.NoBody)
	if err != nil {
		return nil, err
	}
	p.setHeaders(req)

	resp, err := provider.DoWithRetry(ctx, p.retry, func() (*http.Response, error) {
		return p.client.Do(req)
	})
	if err != nil {
		return nil, fmt.Errorf("downloading results: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("results download failed (HTTP %d): %s", resp.StatusCode, body)
	}

	tmpPath := outputPath + ".tmp"
	out, err := os.Create(tmpPath)
	if err != nil {
		return nil, err
	}
	defer out.Close() //nolint:errcheck // closed explicitly before rename
	bw := bufio.NewWriter(out)

	succeeded, failed, expired := 0, 0, 0
	scanner := jsonl.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}

		var result struct {
			Result struct {
				Type string `json:"type"`
			} `json:"result"`
		}
		if json.Unmarshal(line, &result) == nil {
			switch result.Result.Type {
			case "succeeded":
				succeeded++
			case "errored":
				failed++
			case "expired":
				expired++
			}
		}

		if _, err := bw.Write(line); err != nil {
			_ = os.Remove(tmpPath)
			return nil, err
		}
		if _, err := bw.WriteString("\n"); err != nil {
			_ = os.Remove(tmpPath)
			return nil, err
		}
	}
	if err := scanner.Err(); err != nil {
		_ = os.Remove(tmpPath)
		return nil, err
	}
	if err := bw.Flush(); err != nil {
		_ = os.Remove(tmpPath)
		return nil, err
	}

	_ = out.Close()
	if err := os.Rename(tmpPath, outputPath); err != nil {
		return nil, err
	}

	return &provider.ResultSummary{
		OutputPath: outputPath,
		Succeeded:  succeeded,
		Failed:     failed,
		Expired:    expired,
		Total:      succeeded + failed + expired,
	}, nil
}

func (p *Provider) Cancel(ctx context.Context, batchID string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/messages/batches/"+batchID+"/cancel", http.NoBody)
	if err != nil {
		return err
	}
	p.setHeaders(req)
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("cancel failed (HTTP %d): %s", resp.StatusCode, body)
	}
	return nil
}

func (p *Provider) readRequests(path string) ([]json.RawMessage, int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, err
	}

	var requests []json.RawMessage
	scanner := jsonl.NewScannerFromBytes(data)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		cp := make([]byte, len(line))
		copy(cp, line)
		requests = append(requests, json.RawMessage(cp))
	}
	if err := scanner.Err(); err != nil {
		return nil, 0, err
	}
	return requests, len(requests), nil
}

func (p *Provider) setHeaders(req *http.Request) {
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", p.apiVersion)
	req.Header.Set("Content-Type", "application/json")
}

type batchResponse struct {
	ID               string `json:"id"`
	ProcessingStatus string `json:"processing_status"`
	ResultsURL       string `json:"results_url"`
	RequestCounts    struct {
		Processing int `json:"processing"`
		Succeeded  int `json:"succeeded"`
		Errored    int `json:"errored"`
		Canceled   int `json:"canceled"`
		Expired    int `json:"expired"`
	} `json:"request_counts"`
}
