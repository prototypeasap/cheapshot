package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type FileConfig struct {
	Providers map[string]Profile `yaml:"providers"`
	Default   string             `yaml:"default"`
}

type Profile struct {
	BaseURL     string         `yaml:"base_url"`
	Model       string         `yaml:"model"`
	Mode        string         `yaml:"mode"`
	Concurrency int            `yaml:"concurrency"`
	APIKeyEnv   string         `yaml:"api_key_env"`
	Format      string         `yaml:"format"`
	ExtraBody   map[string]any `yaml:"extra_body"`
}

type ResolvedConfig struct {
	Name        string
	Mode        string
	Format      string
	Model       string
	BaseURL     string
	APIKey      string
	Concurrency int
}

func LoadConfig() (*FileConfig, error) {
	path := os.Getenv("CHEAPSHOT_CONFIG")
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return &FileConfig{}, nil //nolint:nilerr // no home dir means no config, not an error
		}
		path = filepath.Join(home, ".cheapshot", "config.yaml")
	}

	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		if os.IsNotExist(err) {
			return &FileConfig{}, nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	warnWorldReadable(path)

	var cfg FileConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	if err := rejectRawKeys(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func ResolveRunConfig(providerFlag, modeFlag string, concurrency int) (*ResolvedConfig, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}

	name, err := resolveProviderFromConfig(providerFlag, cfg)
	if err != nil {
		return nil, err
	}

	rc := &ResolvedConfig{Name: name}
	applyProfile(rc, cfg, name)
	applyBuiltinDefaults(rc, name)

	if rc.APIKey == "" && rc.BaseURL == "" {
		return nil, fmt.Errorf("no API key for provider %q; set the appropriate env var or api_key_env in config", name)
	}

	if modeFlag != "" {
		rc.Mode = modeFlag
	}
	if rc.Mode == "" {
		rc.Mode = "batch"
	}
	if concurrency > 0 {
		rc.Concurrency = concurrency
	}
	if rc.Concurrency == 0 {
		rc.Concurrency = 10
	}

	return rc, nil
}

func resolveProviderFromConfig(flag string, cfg *FileConfig) (string, error) {
	name := flag
	if name == "" {
		name = os.Getenv("CHEAPSHOT_PROVIDER")
	}
	if name == "" {
		name = cfg.Default
	}
	if name == "" {
		return detectProviderName()
	}
	return name, nil
}

func applyProfile(rc *ResolvedConfig, cfg *FileConfig, name string) {
	profile, ok := cfg.Providers[name]
	if !ok {
		return
	}
	rc.Format = profile.Format
	rc.Mode = profile.Mode
	rc.Model = profile.Model
	rc.BaseURL = profile.BaseURL
	rc.Concurrency = profile.Concurrency
	if profile.APIKeyEnv != "" {
		rc.APIKey = os.Getenv(profile.APIKeyEnv)
	}
}

func applyBuiltinDefaults(rc *ResolvedConfig, name string) {
	switch name {
	case "openai":
		if rc.Format == "" {
			rc.Format = "openai"
		}
		if rc.APIKey == "" {
			rc.APIKey = os.Getenv("OPENAI_API_KEY")
		}
		if rc.BaseURL == "" {
			rc.BaseURL = envOrDefault("OPENAI_BASE_URL", "https://api.openai.com")
		}
	case "anthropic":
		if rc.Format == "" {
			rc.Format = "anthropic"
		}
		if rc.APIKey == "" {
			rc.APIKey = os.Getenv("ANTHROPIC_API_KEY")
		}
		if rc.BaseURL == "" {
			rc.BaseURL = envOrDefault("ANTHROPIC_BASE_URL", "https://api.anthropic.com")
		}
	default:
		if rc.Format == "" {
			rc.Format = "openai"
		}
	}
}

type PrepareConfig struct {
	Format    string
	Model     string
	ExtraBody map[string]any
}

func ResolvePrepareConfig(providerFlag, modelFlag string) (*PrepareConfig, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}

	name := providerFlag
	if name == "" {
		name = os.Getenv("CHEAPSHOT_PROVIDER")
	}
	if name == "" {
		name = cfg.Default
	}
	if name == "" {
		name, err = detectProviderName()
		if err != nil {
			return nil, err
		}
	}

	pc := &PrepareConfig{Format: "openai"}
	if name == "anthropic" {
		pc.Format = "anthropic"
	}
	if profile, ok := cfg.Providers[name]; ok {
		if profile.Format != "" {
			pc.Format = profile.Format
		}
		if modelFlag == "" {
			pc.Model = profile.Model
		}
		pc.ExtraBody = profile.ExtraBody
	}

	if modelFlag != "" {
		pc.Model = modelFlag
	}
	if pc.Model == "" {
		return nil, fmt.Errorf("--model is required (no default in config for %q)", name)
	}

	return pc, nil
}

func detectProviderName() (string, error) {
	hasOpenAI := os.Getenv("OPENAI_API_KEY") != ""
	hasAnthropic := os.Getenv("ANTHROPIC_API_KEY") != ""

	if hasOpenAI && hasAnthropic {
		return "", fmt.Errorf("both OPENAI_API_KEY and ANTHROPIC_API_KEY are set; use -p to pick one")
	}
	if hasOpenAI {
		return "openai", nil
	}
	if hasAnthropic {
		return "anthropic", nil
	}
	return "", fmt.Errorf("no API key found; set OPENAI_API_KEY or ANTHROPIC_API_KEY, or configure a provider in ~/.cheapshot/config.yaml")
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func warnWorldReadable(path string) {
	info, err := os.Stat(filepath.Clean(path))
	if err != nil {
		return
	}
	if info.Mode().Perm()&0o044 != 0 {
		fmt.Fprintf(os.Stderr, "warning: %s is readable by others; consider: chmod 600 %s\n", path, path)
	}
}

func rejectRawKeys(cfg *FileConfig) error {
	for name, profile := range cfg.Providers {
		if looksLikeKey(profile.APIKeyEnv) {
			return fmt.Errorf(
				"provider %q: api_key_env looks like a raw API key, not an env var name; "+
					"set api_key_env to the NAME of an environment variable (e.g. MY_API_KEY), not the key itself",
				name,
			)
		}
	}
	return nil
}

func looksLikeKey(s string) bool {
	return strings.HasPrefix(s, "sk-") || strings.HasPrefix(s, "sk-ant-")
}
