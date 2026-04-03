package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	OpenAIKey     string `json:"openai_key,omitempty"`
	OpenAIModel   string `json:"openai_model,omitempty"`
	AnthropicKey  string `json:"anthropic_key,omitempty"`
	AnthropicModel string `json:"anthropic_model,omitempty"`
}

func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".sekd")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

func Load() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return &Config{}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return &Config{}, nil
	}
	var cfg Config
	json.Unmarshal(data, &cfg)
	return &cfg, nil
}

func (c *Config) Save() error {
	path, err := configPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func (c *Config) Set(key, value string) bool {
	switch strings.ToLower(key) {
	case "openai-key", "openai_key", "openai":
		c.OpenAIKey = value
	case "openai-model", "openai_model":
		c.OpenAIModel = value
	case "anthropic-key", "anthropic_key", "anthropic":
		c.AnthropicKey = value
	case "anthropic-model", "anthropic_model":
		c.AnthropicModel = value
	default:
		return false
	}
	return true
}

func (c *Config) Clear(key string) bool {
	return c.Set(key, "")
}

// Apply loads config values into environment variables
// (env vars take precedence over config file)
func (c *Config) Apply() {
	if os.Getenv("OPENAI_API_KEY") == "" && c.OpenAIKey != "" {
		os.Setenv("OPENAI_API_KEY", c.OpenAIKey)
	}
	if os.Getenv("OPENAI_MODEL") == "" && c.OpenAIModel != "" {
		os.Setenv("OPENAI_MODEL", c.OpenAIModel)
	}
	if os.Getenv("ANTHROPIC_API_KEY") == "" && c.AnthropicKey != "" {
		os.Setenv("ANTHROPIC_API_KEY", c.AnthropicKey)
	}
	if os.Getenv("ANTHROPIC_MODEL") == "" && c.AnthropicModel != "" {
		os.Setenv("ANTHROPIC_MODEL", c.AnthropicModel)
	}
}

func MaskKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "..." + key[len(key)-4:]
}
