package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnvironmentPriorityAndIsolation(t *testing.T) {
	root := t.TempDir()
	for name, data := range map[string]string{
		".env":                   "ENV_TEST_SHARED=base\n",
		".env.development":       "ENV_TEST_MODE=development\nENV_TEST_PRIORITY=default\n",
		".env.test":              "ENV_TEST_MODE=test\nENV_TEST_PRIORITY=test-default\n",
		".env.local":             "ENV_TEST_PRIORITY=development-local\nENV_TEST_LOCAL=private\n",
		".env.development.local": "ENV_TEST_PRIORITY=mode-local\n",
		".env.test.local":        "ENV_TEST_OVERRIDE=quoted\n",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	get, err := environmentAt(root, "development")
	if err != nil {
		t.Fatal(err)
	}
	if get("ENV_TEST_PRIORITY") != "mode-local" || get("ENV_TEST_SHARED") != "base" {
		t.Fatal("incorrect development priority")
	}
	get, err = environmentAt(root, "test")
	if err != nil {
		t.Fatal(err)
	}
	if get("ENV_TEST_PRIORITY") != "test-default" || get("ENV_TEST_LOCAL") != "" || get("ENV_TEST_MODE") != "test" {
		t.Fatal("test environment leaked development settings")
	}
	t.Setenv("ENV_TEST_PRIORITY", "process")
	if get("ENV_TEST_PRIORITY") != "process" {
		t.Fatal("process override ignored")
	}
	t.Setenv("ENV_TEST_PRIORITY", "")
	if get("ENV_TEST_PRIORITY") != "" {
		t.Fatal("empty process override ignored")
	}
	if _, exists := os.LookupEnv("ENV_TEST_MODE"); exists {
		t.Fatal("loader mutated process environment")
	}
}

func TestEnvironmentErrors(t *testing.T) {
	root := t.TempDir()
	if _, err := environmentAt(root, "production"); err == nil {
		t.Fatal("unsupported mode accepted")
	}
	if _, err := environmentAt(root, "test"); err == nil || !strings.Contains(err.Error(), ".env.test") {
		t.Fatal("missing file accepted", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".env.test"), []byte("INVALID LINE\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := environmentAt(root, "test"); err == nil {
		t.Fatal("malformed file accepted")
	}
}
