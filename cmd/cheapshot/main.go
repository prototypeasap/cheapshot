package main

import (
	"fmt"
	"os"

	"github.com/prototypeasap/cheapshot/internal/cli"
	"github.com/spf13/cobra"

	_ "modernc.org/sqlite"
)

var version = "dev"

func main() {
	root := &cobra.Command{
		Use:     "cheapshot",
		Short:   "CLI for LLM batch APIs",
		Long:    "CLI for the OpenAI and Anthropic batch APIs.\nhttps://github.com/prototypeasap/cheapshot",
		Version: version,
	}

	root.AddCommand(
		cli.NewPrepareCmd(),
		cli.NewSubmitCmd(),
		cli.NewStatusCmd(),
		cli.NewResultsCmd(),
		cli.NewRunCmd(),
		cli.NewListCmd(),
		cli.NewCancelCmd(),
		cli.NewRecoverCmd(),
		cli.NewExtractCmd(),
	)

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
