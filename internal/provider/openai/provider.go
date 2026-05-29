package openai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/prototypeasap/cheapshot/internal/jsonl"
	"github.com/prototypeasap/cheapshot/internal/provider"
)

type Provider struct {
	apiKey  string
	baseURL string
	client  *http.Client
	retry   provider.RetryConfig
}

func New(apiKey, baseURL string) *Provider {
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	return &Provider{
		apiKey:  apiKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 5 * time.Minute},
		retry:   provider.DefaultRetryConfig(),
	}
}

func (p *Provider) Name() string { return "openai" }

func (p *Provider) Validate(input []byte) error {
	return jsonl.ValidateOpenAI(input)
}

func (p *Provider) Submit(ctx context.Context, inputPath string, opts provider.SubmitOpts) (*provider.SubmitResult, error) {
	count, err := jsonl.CountLines(inputPath)
	if err != nil {
		return nil, fmt.Errorf("counting lines: %w", err)
	}

	fileID, err := p.uploadFile(ctx, inputPath)
	if err != nil {
		return nil, fmt.Errorf("uploading file: %w", err)
	}

	endpoint := opts.Endpoint
	if endpoint == "" {
		endpoint = "/v1/chat/completions"
	}
	window := opts.CompletionWindow
	if window == "" {
		window = "24h"
	}

	batchID, err := p.createBatch(ctx, fileID, endpoint, window)
	if err != nil {
		return nil, fmt.Errorf("creating batch (file_id=%s): %w", fileID, err)
	}

	return &provider.SubmitResult{
		RemoteBatchID: batchID,
		RemoteFileID:  fileID,
		RequestCount:  count,
	}, nil
}

func (p *Provider) Status(ctx context.Context, batchID string) (*provider.BatchStatus, error) {
	batch, err := p.getBatch(ctx, batchID)
	if err != nil {
		return nil, err
	}

	terminal := false
	switch batch.Status {
	case "completed", "failed", "expired", "cancelled":
		terminal = true
	}

	return &provider.BatchStatus{
		Raw:       batch.Status,
		Terminal:  terminal,
		Succeeded: batch.RequestCounts.Completed,
		Failed:    batch.RequestCounts.Failed,
		Total:     batch.RequestCounts.Total,
	}, nil
}

func (p *Provider) getBatch(ctx context.Context, batchID string) (*batchResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/v1/batches/"+batchID, http.NoBody)
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
	return &batch, nil
}

func (p *Provider) Results(ctx context.Context, batchID, outputPath string) (*provider.ResultSummary, error) {
	batch, err := p.getBatch(ctx, batchID)
	if err != nil {
		return nil, err
	}

	terminal := false
	switch batch.Status {
	case "completed", "failed", "expired", "cancelled":
		terminal = true
	}
	if !terminal {
		return nil, fmt.Errorf("batch %s is not terminal (status: %s)", batchID, batch.Status)
	}

	tmpPath := outputPath + ".tmp"
	out, err := os.Create(tmpPath)
	if err != nil {
		return nil, err
	}
	defer out.Close() //nolint:errcheck // closed explicitly before rename
	bw := bufio.NewWriter(out)

	succeeded, failed := 0, 0

	if batch.OutputFileID != "" {
		s, f, err := p.downloadAndWrite(ctx, batch.OutputFileID, bw)
		if err != nil {
			_ = os.Remove(tmpPath)
			return nil, fmt.Errorf("downloading output file: %w", err)
		}
		succeeded += s
		failed += f
	}

	if batch.ErrorFileID != "" {
		s, f, err := p.downloadAndWrite(ctx, batch.ErrorFileID, bw)
		if err != nil {
			_ = os.Remove(tmpPath)
			return nil, fmt.Errorf("downloading error file: %w", err)
		}
		succeeded += s
		failed += f
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
		Total:      succeeded + failed,
	}, nil
}

func (p *Provider) Cancel(ctx context.Context, batchID string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/batches/"+batchID+"/cancel", http.NoBody)
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

func (p *Provider) uploadFile(ctx context.Context, path string) (string, error) {
	fileData, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	baseName := filepath.Base(path)

	resp, err := provider.DoWithRetry(ctx, p.retry, func() (*http.Response, error) {
		var buf bytes.Buffer
		w := multipart.NewWriter(&buf)
		if err := w.WriteField("purpose", "batch"); err != nil {
			return nil, err
		}
		part, err := w.CreateFormFile("file", baseName)
		if err != nil {
			return nil, err
		}
		if _, err := part.Write(fileData); err != nil {
			return nil, err
		}
		_ = w.Close()

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/files", &buf)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
		req.Header.Set("Content-Type", w.FormDataContentType())
		return p.client.Do(req)
	})
	if err != nil {
		return "", fmt.Errorf("upload failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("upload failed (HTTP %d): %s", resp.StatusCode, body)
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return result.ID, nil
}

func (p *Provider) createBatch(ctx context.Context, fileID, endpoint, window string) (string, error) {
	body, _ := json.Marshal(map[string]string{
		"input_file_id":     fileID,
		"endpoint":          endpoint,
		"completion_window": window,
	})

	resp, err := provider.DoWithRetry(ctx, p.retry, func() (*http.Response, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/batches", bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		p.setHeaders(req)
		return p.client.Do(req)
	})
	if err != nil {
		return "", fmt.Errorf("create batch failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("create batch failed (HTTP %d): %s", resp.StatusCode, respBody)
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return result.ID, nil
}

func (p *Provider) downloadFile(ctx context.Context, fileID string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/v1/files/"+fileID+"/content", http.NoBody)
	if err != nil {
		return nil, err
	}
	p.setHeaders(req)

	resp, err := provider.DoWithRetry(ctx, p.retry, func() (*http.Response, error) {
		return p.client.Do(req)
	})
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		return nil, fmt.Errorf("download failed (HTTP %d): %s", resp.StatusCode, body)
	}
	return resp.Body, nil
}

func (p *Provider) downloadAndWrite(ctx context.Context, fileID string, out io.Writer) (succeeded, failed int, err error) {
	body, err := p.downloadFile(ctx, fileID)
	if err != nil {
		return 0, 0, err
	}
	defer body.Close()

	scanner := jsonl.NewScanner(body)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}

		var result struct {
			Error *json.RawMessage `json:"error"`
		}
		if json.Unmarshal(line, &result) == nil && result.Error != nil && string(*result.Error) != "null" {
			failed++
		} else {
			succeeded++
		}

		if _, err := out.Write(line); err != nil {
			return succeeded, failed, err
		}
		if _, err := fmt.Fprintln(out); err != nil {
			return succeeded, failed, err
		}
	}
	return succeeded, failed, scanner.Err()
}

func (p *Provider) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")
}

type batchResponse struct {
	ID            string `json:"id"`
	Status        string `json:"status"`
	OutputFileID  string `json:"output_file_id"`
	ErrorFileID   string `json:"error_file_id"`
	RequestCounts struct {
		Total     int `json:"total"`
		Completed int `json:"completed"`
		Failed    int `json:"failed"`
	} `json:"request_counts"`
}
