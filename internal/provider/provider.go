package provider

import "context"

type Provider interface {
	Name() string
	Validate(input []byte) error
	Submit(ctx context.Context, inputPath string, opts SubmitOpts) (*SubmitResult, error)
	Status(ctx context.Context, batchID string) (*BatchStatus, error)
	Results(ctx context.Context, batchID string, outputPath string) (*ResultSummary, error)
	Cancel(ctx context.Context, batchID string) error
}

type SubmitOpts struct {
	Endpoint         string
	CompletionWindow string
}

type SubmitResult struct {
	RemoteBatchID string
	RemoteFileID  string // OpenAI only
	RequestCount  int
}

type BatchStatus struct {
	Raw       string
	Terminal  bool
	Succeeded int
	Failed    int
	Expired   int
	Canceled  int
	Total     int
	ResultURL string // Anthropic only
}

type ResultSummary struct {
	OutputPath string
	Succeeded  int
	Failed     int
	Expired    int
	Total      int
}
