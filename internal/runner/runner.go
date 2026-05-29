package runner

import "context"

type Runner interface {
	Run(ctx context.Context, inputPath, outputPath string, opts RunOpts) (*RunResult, error)
}

type RunOpts struct {
	Endpoint         string
	CompletionWindow string
	PollInterval     int
}

type RunResult struct {
	Succeeded int
	Failed    int
	Total     int
}
