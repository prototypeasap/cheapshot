package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/prototypeasap/cheapshot/internal/provider"
	"github.com/prototypeasap/cheapshot/internal/provider/anthropic"
	"github.com/prototypeasap/cheapshot/internal/provider/openai"
)

func ResolveProvider(providerFlag string, timeout time.Duration) (provider.Provider, error) {
	p := providerFlag
	if p == "" {
		p = os.Getenv("CHEAPSHOT_PROVIDER")
	}

	openaiKey := os.Getenv("OPENAI_API_KEY")
	anthropicKey := os.Getenv("ANTHROPIC_API_KEY")

	switch strings.ToLower(p) {
	case "openai":
		if openaiKey == "" {
			return nil, fmt.Errorf("OPENAI_API_KEY not set")
		}
		return openai.New(openaiKey, os.Getenv("OPENAI_BASE_URL"), timeout), nil
	case "anthropic":
		if anthropicKey == "" {
			return nil, fmt.Errorf("ANTHROPIC_API_KEY not set")
		}
		return anthropic.New(anthropicKey, os.Getenv("ANTHROPIC_BASE_URL"), timeout), nil
	case "":
		return autoDetect(openaiKey, anthropicKey, timeout)
	default:
		return nil, fmt.Errorf("unknown provider %q (use 'openai' or 'anthropic')", p)
	}
}

func autoDetect(openaiKey, anthropicKey string, timeout time.Duration) (provider.Provider, error) {
	hasOpenAI := openaiKey != ""
	hasAnthropic := anthropicKey != ""

	if hasOpenAI && hasAnthropic {
		return nil, fmt.Errorf("both OPENAI_API_KEY and ANTHROPIC_API_KEY are set; use -p to pick one")
	}
	if hasOpenAI {
		return openai.New(openaiKey, os.Getenv("OPENAI_BASE_URL"), timeout), nil
	}
	if hasAnthropic {
		return anthropic.New(anthropicKey, os.Getenv("ANTHROPIC_BASE_URL"), timeout), nil
	}
	return nil, fmt.Errorf("no API key found; set OPENAI_API_KEY or ANTHROPIC_API_KEY")
}

func ResolveProviderName(providerFlag string) (string, error) {
	p := providerFlag
	if p == "" {
		p = os.Getenv("CHEAPSHOT_PROVIDER")
	}
	switch strings.ToLower(p) {
	case "openai", "anthropic":
		return strings.ToLower(p), nil
	case "":
		openaiKey := os.Getenv("OPENAI_API_KEY")
		anthropicKey := os.Getenv("ANTHROPIC_API_KEY")
		if openaiKey != "" && anthropicKey != "" {
			return "", fmt.Errorf("both OPENAI_API_KEY and ANTHROPIC_API_KEY are set; use -p to pick one")
		}
		if openaiKey != "" {
			return "openai", nil
		}
		if anthropicKey != "" {
			return "anthropic", nil
		}
		return "", fmt.Errorf("no API key found; set OPENAI_API_KEY or ANTHROPIC_API_KEY")
	default:
		return "", fmt.Errorf("unknown provider %q", p)
	}
}

func DBPath() string {
	if p := os.Getenv("CHEAPSHOT_DB"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".cheapshot", "cheapshot.db")
}
