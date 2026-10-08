package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

// Environment captures file values without changing process variables.
// Its lookup reads process overrides on each call, including explicit empty values.
func Environment(mode string) (func(string) string, error) {
	root, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	for {
		if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(root)
		if parent == root {
			return nil, fmt.Errorf("backend project root not found; run from backend/")
		}
		root = parent
	}
	return environmentAt(root, mode)
}

func environmentAt(root, mode string) (func(string) string, error) {
	if mode == "" {
		mode = "development"
	}
	if mode != "development" && mode != "test" {
		return nil, fmt.Errorf("unsupported APP_ENV %q: use development or test", mode)
	}
	files := []string{".env." + mode + ".local"}
	if mode != "test" {
		// Personal development overrides must not leak into isolated test runs.
		files = append(files, ".env.local")
	}
	files = append(files, ".env."+mode, ".env")
	values := map[string]string{}
	for _, file := range files {
		data, err := godotenv.Read(filepath.Join(root, file))
		// A missing mode file must not silently fall back to generic settings.
		if os.IsNotExist(err) && file != ".env."+mode {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("load %s: %w", file, err)
		}
		for key, value := range data {
			if _, exists := values[key]; !exists {
				values[key] = value
			}
		}
	}
	return func(key string) string {
		// Falling back from an empty override would mask invalid configuration.
		if value, exists := os.LookupEnv(key); exists {
			return value
		}
		return values[key]
	}, nil
}
