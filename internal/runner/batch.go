package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/prototypeasap/cheapshot/internal/provider"
	"github.com/prototypeasap/cheapshot/internal/store"
)

type BatchRunner struct {
	provider    provider.Provider
	db          *store.Store
	displayName string
}

func NewBatchRunner(prov provider.Provider, db *store.Store, displayName string) *BatchRunner {
	return &BatchRunner{provider: prov, db: db, displayName: displayName}
}

func (r *BatchRunner) Run(ctx context.Context, inputPath, outputPath string, opts RunOpts) (*RunResult, error) {
	fmt.Fprintf(os.Stderr, "Submitting to %s...\n", r.provider.Name())

	endpoint := opts.Endpoint
	window := opts.CompletionWindow

	result, err := r.provider.Submit(ctx, inputPath, provider.SubmitOpts{
		Endpoint:         endpoint,
		CompletionWindow: window,
	})
	if err != nil {
		return nil, err
	}

	batchDBID, err := r.db.CreateBatch(r.provider.Name(), r.displayName, result.RequestCount, endpoint)
	if err != nil {
		return nil, err
	}
	if result.RemoteFileID != "" {
		_ = r.db.MarkUploaded(batchDBID, result.RemoteFileID)
	}
	_ = r.db.MarkSubmitted(batchDBID, result.RemoteBatchID)

	fmt.Fprintf(os.Stderr, "Batch %s submitted (%d requests)\n", result.RemoteBatchID, result.RequestCount)

	pollInterval := time.Duration(opts.PollInterval) * time.Second
	if pollInterval == 0 {
		pollInterval = 15 * time.Second
	}

	return r.pollAndDownload(ctx, batchDBID, result.RemoteBatchID, outputPath, pollInterval)
}

func (r *BatchRunner) pollAndDownload(ctx context.Context, batchDBID int64, remoteBatchID, outputPath string, pollInterval time.Duration) (*RunResult, error) {
	for {
		select {
		case <-ctx.Done():
			r.printInterrupted(remoteBatchID, outputPath)
			return nil, nil //nolint:nilerr // graceful exit on Ctrl+C
		default:
		}

		status, err := r.provider.Status(ctx, remoteBatchID)
		if err != nil {
			if ctx.Err() != nil {
				fmt.Fprintf(os.Stderr, "\nInterrupted. Batch %s continues on %s.\n", remoteBatchID, r.provider.Name())
				return nil, nil //nolint:nilerr // graceful exit on Ctrl+C
			}
			return nil, err
		}

		_ = r.db.UpdatePollStatus(batchDBID, status.Raw)

		fmt.Fprintf(os.Stderr, "Polling... %s (%d/%d completed)\n", status.Raw, status.Succeeded+status.Failed, status.Total)

		if status.Terminal {
			_ = r.db.MarkCompleted(batchDBID, status.Succeeded, status.Failed)

			fmt.Fprintf(os.Stderr, "Downloading results...\n")
			summary, err := r.provider.Results(ctx, remoteBatchID, outputPath)
			if err != nil {
				return nil, err
			}

			_ = r.db.MarkDownloaded(batchDBID, outputPath, summary.Succeeded, summary.Failed)

			fmt.Fprintf(os.Stderr, "Done. %d succeeded, %d failed. Results: %s\n", summary.Succeeded, summary.Failed, outputPath)

			return &RunResult{
				Succeeded: summary.Succeeded,
				Failed:    summary.Failed,
				Total:     summary.Total,
			}, nil
		}

		select {
		case <-ctx.Done():
			fmt.Fprintf(os.Stderr, "\nInterrupted. Batch %s continues on %s.\n", remoteBatchID, r.provider.Name())
			return nil, nil //nolint:nilerr // graceful exit on Ctrl+C
		case <-time.After(pollInterval):
		}
	}
}

func (r *BatchRunner) printInterrupted(remoteBatchID, outputPath string) {
	fmt.Fprintf(os.Stderr, "\nInterrupted. Batch %s is still running on %s.\n", remoteBatchID, r.provider.Name())
	fmt.Fprintf(os.Stderr, "Resume later:\n")
	fmt.Fprintf(os.Stderr, "  cheapshot status -b %s --watch\n", remoteBatchID)

	if outputPath != "" {
		fmt.Fprintf(os.Stderr, "  cheapshot results -b %s -o %s\n", remoteBatchID, outputPath)
	}
}

func PrintRunResult(providerName, outputPath string, result *RunResult) error {
	out := map[string]any{
		"provider":    providerName,
		"output_file": outputPath,
		"succeeded":   result.Succeeded,
		"failed":      result.Failed,
		"total":       result.Total,
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
