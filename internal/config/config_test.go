package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_CorruptFileReturnsError(t *testing.T) {
	// Regression: Load used to silently discard a corrupt config,
	// leaving the user wondering why their API key "disappeared".
	// It must surface the error so the caller can warn the user.
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	sekdDir := filepath.Join(dir, ".sekd")
	if err := os.MkdirAll(sekdDir, 0700); err != nil {
		t.Fatalf("setup mkdir: %v", err)
	}
	path := filepath.Join(sekdDir, "config.json")
	if err := os.WriteFile(path, []byte("this is not json {"), 0600); err != nil {
		t.Fatalf("setup write: %v", err)
	}

	cfg, err := Load()
	if err == nil {
		t.Fatal("Load should return an error for corrupt JSON")
	}
	if cfg == nil {
		t.Fatal("Load should still return a non-nil (empty) config on error")
	}
	if cfg.OpenAIKey != "" {
		t.Errorf("corrupt config should yield empty key, got %q", cfg.OpenAIKey)
	}
}

func TestSetAndSaveAndLoad(t *testing.T) {
	// Use temp dir
	dir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	defer os.Setenv("HOME", origHome)

	cfg := &Config{}
	cfg.Set("openai-key", "sk-test-123456789012345678901234567890")
	cfg.Set("openai-model", "gpt-4o")

	if cfg.OpenAIKey != "sk-test-123456789012345678901234567890" {
		t.Errorf("OpenAIKey = %q, want sk-test-...", cfg.OpenAIKey)
	}
	if cfg.OpenAIModel != "gpt-4o" {
		t.Errorf("OpenAIModel = %q, want gpt-4o", cfg.OpenAIModel)
	}

	if err := cfg.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file exists
	path := filepath.Join(dir, ".sekd", "config.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("config.json was not created")
	}

	// Verify permissions (should be 0600)
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0600 {
		t.Errorf("permissions = %o, want 600", info.Mode().Perm())
	}

	// Load and verify
	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.OpenAIKey != "sk-test-123456789012345678901234567890" {
		t.Errorf("Loaded OpenAIKey = %q", loaded.OpenAIKey)
	}
	if loaded.OpenAIModel != "gpt-4o" {
		t.Errorf("Loaded OpenAIModel = %q", loaded.OpenAIModel)
	}
}

func TestClear(t *testing.T) {
	cfg := &Config{OpenAIKey: "sk-test"}
	cfg.Clear("openai-key")
	if cfg.OpenAIKey != "" {
		t.Errorf("OpenAIKey should be empty after clear, got %q", cfg.OpenAIKey)
	}
}

func TestSetUnknownKey(t *testing.T) {
	cfg := &Config{}
	if cfg.Set("unknown-key", "value") {
		t.Error("Set should return false for unknown key")
	}
}

func TestSetAlternateKeyNames(t *testing.T) {
	tests := []struct {
		key    string
		field  string
	}{
		{"openai-key", "openai"},
		{"openai_key", "openai"},
		{"openai", "openai"},
		{"anthropic-key", "anthropic"},
		{"anthropic_key", "anthropic"},
		{"anthropic", "anthropic"},
	}

	for _, tt := range tests {
		cfg := &Config{}
		if !cfg.Set(tt.key, "test-value") {
			t.Errorf("Set(%q) returned false", tt.key)
		}
	}
}

func TestMaskKey(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"sk-proj-abcdefghijklmnop", "sk-p...mnop"},
		{"short", "****"},
		{"12345678", "****"},
		{"123456789", "1234...6789"},
	}

	for _, tt := range tests {
		got := MaskKey(tt.input)
		if got != tt.want {
			t.Errorf("MaskKey(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestApply(t *testing.T) {
	// Clean env
	os.Unsetenv("OPENAI_API_KEY")
	os.Unsetenv("OPENAI_MODEL")

	cfg := &Config{
		OpenAIKey:   "sk-from-config",
		OpenAIModel: "gpt-4",
	}
	cfg.Apply()

	if got := os.Getenv("OPENAI_API_KEY"); got != "sk-from-config" {
		t.Errorf("OPENAI_API_KEY = %q, want sk-from-config", got)
	}

	// Env should take precedence
	os.Setenv("OPENAI_API_KEY", "sk-from-env")
	cfg2 := &Config{OpenAIKey: "sk-from-config-2"}
	cfg2.Apply()

	if got := os.Getenv("OPENAI_API_KEY"); got != "sk-from-env" {
		t.Errorf("OPENAI_API_KEY should keep env value, got %q", got)
	}

	// Cleanup
	os.Unsetenv("OPENAI_API_KEY")
	os.Unsetenv("OPENAI_MODEL")
}

func TestLoadNonExistent(t *testing.T) {
	dir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	defer os.Setenv("HOME", origHome)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load should not fail for missing file: %v", err)
	}
	if cfg.OpenAIKey != "" {
		t.Error("should return empty config for missing file")
	}
}
