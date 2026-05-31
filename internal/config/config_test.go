package config

import (
	"os"
	"testing"

	"github.com/go-pkgz/lgr"
)

func newTestLogger() *lgr.Logger {
	return lgr.New(lgr.Debug, lgr.CallerFunc)
}

func TestNewConfig_FileEnvVars(t *testing.T) {
	apiKeyFile := t.TempDir() + "/api_key"
	appUrlFile := t.TempDir() + "/app_url"
	os.WriteFile(apiKeyFile, []byte("key_from_file\n"), 0600)
	os.WriteFile(appUrlFile, []byte("http://ff.local\n"), 0600)

	os.Setenv("FF_API_KEY_FILE", apiKeyFile)
	os.Setenv("FF_APP_URL_FILE", appUrlFile)
	t.Cleanup(func() {
		os.Unsetenv("FF_API_KEY_FILE")
		os.Unsetenv("FF_APP_URL_FILE")
	})

	cfg, err := NewConfig(newTestLogger())

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if cfg.APIKey != "key_from_file" {
		t.Errorf("expected APIKey 'key_from_file', got %q", cfg.APIKey)
	}
	if cfg.FFApp != "http://ff.local" {
		t.Errorf("expected FFApp 'http://ff.local', got %q", cfg.FFApp)
	}
}

func TestNewConfig_MissingEnvVars(t *testing.T) {
	os.Unsetenv("FF_API_KEY")
	os.Unsetenv("FF_API_KEY_FILE")
	os.Unsetenv("FF_APP_URL")
	os.Unsetenv("FF_APP_URL_FILE")

	cfg, err := NewConfig(newTestLogger())

	if err == nil {
		t.Fatal("expected error for missing env vars, got nil")
	}
	if cfg != nil {
		t.Error("expected nil config when env vars are missing")
	}
}

func TestNewConfig_DirectEnvVars(t *testing.T) {
	os.Setenv("FF_API_KEY", "test_api_key")
	os.Setenv("FF_APP_URL", "http://firefly.local")
	t.Cleanup(func() {
		os.Unsetenv("FF_API_KEY")
		os.Unsetenv("FF_APP_URL")
	})

	cfg, err := NewConfig(newTestLogger())

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if cfg.APIKey != "test_api_key" {
		t.Errorf("expected APIKey 'test_api_key', got %q", cfg.APIKey)
	}
	if cfg.FFApp != "http://firefly.local" {
		t.Errorf("expected FFApp 'http://firefly.local', got %q", cfg.FFApp)
	}
}
