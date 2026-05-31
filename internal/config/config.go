package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/go-pkgz/lgr"
)

const (
	FireflyAppTimeout = 10
	ModelFile         = "data/model.gob"
	apiKeyEnvVar      = "FF_API_KEY"
	appUrlEnvVar      = "FF_APP_URL"
)

type Config struct {
	APIKey string
	FFApp  string
}

func NewConfig(logger *lgr.Logger) (*Config, error) {
	apiKey, ok := LookupEnvVar(apiKeyEnvVar, logger)
	if !ok {
		return nil, errors.New(formatMissingEnvError(apiKeyEnvVar))
	}

	appUrl, ok := LookupEnvVar(appUrlEnvVar, logger)
	if !ok {
		return nil, errors.New(formatMissingEnvError(appUrlEnvVar))
	}

	return &Config{APIKey: apiKey, FFApp: appUrl}, nil
}

func LookupEnvVar(name string, logger *lgr.Logger) (string, bool) {
	if fileEnv := os.Getenv(name + "_FILE"); fileEnv != "" {
		return readFile(fileEnv, logger)
	}
	return os.LookupEnv(name)
}

func readFile(path string, logger *lgr.Logger) (string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		logger.Logf("ERROR reading file %s: %v", path, err)
		return "", false
	}
	return strings.TrimSuffix(string(data), "\n"), true
}

func formatMissingEnvError(name string) string {
	return fmt.Sprintf("environment vars %q or %q not set", name, name+"_FILE")
}
