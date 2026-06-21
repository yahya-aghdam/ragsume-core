package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadProfile(t *testing.T) {
	t.Run("loads valid profile", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "profile.yaml")
		content := "name: Test User\nheadline: Go Developer\nsummary: Experienced in backend\nskills:\n  - Go\n  - gRPC\ncontact:\n  email: test@example.com\n"
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("write profile: %v", err)
		}

		profile, err := LoadProfile(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if profile.Name != "Test User" {
			t.Fatalf("got name %q, want Test User", profile.Name)
		}
		if profile.Headline != "Go Developer" {
			t.Fatalf("got headline %q, want Go Developer", profile.Headline)
		}
		if len(profile.Skills) != 2 || profile.Skills[0] != "Go" {
			t.Fatalf("got skills %v, want [Go gRPC]", profile.Skills)
		}
	})

	t.Run("fails on missing file", func(t *testing.T) {
		_, err := LoadProfile("/nonexistent/path.yaml")
		if err == nil {
			t.Fatal("expected error for missing file")
		}
	})

	t.Run("fails on empty name", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "empty.yaml")
		content := "name: \"\"\nheadline: Test\n"
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("write: %v", err)
		}

		_, err := LoadProfile(path)
		if err == nil {
			t.Fatal("expected error for empty name")
		}
		if !strings.Contains(err.Error(), "profile name is required") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("fails on invalid YAML", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "bad.yaml")
		content := "name: [invalid"
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("write: %v", err)
		}

		_, err := LoadProfile(path)
		if err == nil {
			t.Fatal("expected error for invalid YAML")
		}
	})
}

func TestRenderProfileYAML(t *testing.T) {
	t.Run("renders profile", func(t *testing.T) {
		profile := Profile{
			Name:     "Test User",
			Headline: "Go Developer",
			Summary:  "Experienced",
			Skills:   []string{"Go", "gRPC"},
			Contact:  map[string]string{"email": "test@example.com"},
		}

		yaml, err := RenderProfileYAML(profile)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(yaml, "Test User") {
			t.Fatalf("expected Test User in output, got %q", yaml)
		}
		if !strings.Contains(yaml, "Go") {
			t.Fatalf("expected Go in output, got %q", yaml)
		}
	})
}
