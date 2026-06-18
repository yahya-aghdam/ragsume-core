package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setRequiredEnv(t *testing.T) {
	t.Helper()
	t.Setenv("APP_NAME", "test-app")
	t.Setenv("PORT", "3000")
	t.Setenv("DEBUG", "false")
	t.Setenv("RATE", "2.5")
	t.Setenv("QDRANT_URL", "http://localhost:6334")
	t.Setenv("QDRANT_API_KEY", "test-qdrant-key")
	t.Setenv("OPENROUTER_API_KEY", "test-openrouter-key")
	t.Setenv("OLLAMA_URL", "http://localhost:11434")
	t.Setenv("ALLOWED_ORIGIN", "http://localhost:3000")
}

func TestLoad(t *testing.T) {
	t.Run("loads valid environment into global config", func(t *testing.T) {
		setRequiredEnv(t)

		if err := Load(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		want := Config{
			AppName:          "test-app",
			Port:             3000,
			Debug:            false,
			Rate:             2.5,
			LogLevel:         "info",
			LogFormat:        "text",
			QdrantURL:        "http://localhost:6334",
			QdrantAPIKey:     "test-qdrant-key",
			OpenRouterAPIKey: "test-openrouter-key",
			OllamaURL:        "http://localhost:11434",
			AllowedOrigin:    "http://localhost:3000",
		}
		if C != want {
			t.Fatalf("got %+v, want %+v", C, want)
		}
	})

	t.Run("loads from .env file", func(t *testing.T) {
		dir := t.TempDir()
		envFile := filepath.Join(dir, ".env")
		content := "APP_NAME=from-dotenv\nPORT=9090\nDEBUG=true\nRATE=0.75\n" +
			"QDRANT_URL=http://qdrant:6334\nQDRANT_API_KEY=dotenv-key\n" +
			"OPENROUTER_API_KEY=dotenv-or-key\nOLLAMA_URL=http://ollama:11434\n" +
			"ALLOWED_ORIGIN=https://app.example.com\n"
		if err := os.WriteFile(envFile, []byte(content), 0o600); err != nil {
			t.Fatalf("write .env: %v", err)
		}

		oldWD, err := os.Getwd()
		if err != nil {
			t.Fatalf("getwd: %v", err)
		}
		if err := os.Chdir(dir); err != nil {
			t.Fatalf("chdir: %v", err)
		}
		t.Cleanup(func() { _ = os.Chdir(oldWD) })

		for _, key := range []string{
			"APP_NAME", "PORT", "DEBUG", "RATE",
			"QDRANT_URL", "QDRANT_API_KEY", "OPENROUTER_API_KEY",
			"OLLAMA_URL", "ALLOWED_ORIGIN",
		} {
			unsetEnv(t, key)
		}

		if err := Load(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		want := Config{
			AppName:          "from-dotenv",
			Port:             9090,
			Debug:            true,
			Rate:             0.75,
			LogLevel:         "debug",
			LogFormat:        "text",
			QdrantURL:        "http://qdrant:6334",
			QdrantAPIKey:     "dotenv-key",
			OpenRouterAPIKey: "dotenv-or-key",
			OllamaURL:        "http://ollama:11434",
			AllowedOrigin:    "https://app.example.com",
		}
		if C != want {
			t.Fatalf("got %+v, want %+v", C, want)
		}
	})

	t.Run("fails when required variable is missing", func(t *testing.T) {
		setRequiredEnv(t)
		unsetEnv(t, "APP_NAME")

		err := Load()
		if err == nil {
			t.Fatal("expected error for missing APP_NAME")
		}
		if !strings.Contains(err.Error(), `APP_NAME`) {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("fails on invalid port", func(t *testing.T) {
		setRequiredEnv(t)
		t.Setenv("PORT", "not-int")

		err := Load()
		if err == nil {
			t.Fatal("expected error for invalid PORT")
		}
		if !strings.Contains(err.Error(), `must be an integer`) {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("fails on invalid debug", func(t *testing.T) {
		setRequiredEnv(t)
		t.Setenv("DEBUG", "maybe")

		err := Load()
		if err == nil {
			t.Fatal("expected error for invalid DEBUG")
		}
		if !strings.Contains(err.Error(), `must be a boolean`) {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("fails on invalid rate", func(t *testing.T) {
		setRequiredEnv(t)
		t.Setenv("RATE", "bad")

		err := Load()
		if err == nil {
			t.Fatal("expected error for invalid RATE")
		}
		if !strings.Contains(err.Error(), `must be a float`) {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
