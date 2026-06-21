package main

import (
	"strings"
	"testing"
)

func TestBuildSystemPrompt(t *testing.T) {
	t.Run("includes profile content", func(t *testing.T) {
		prompt := BuildSystemPrompt("name: Test User\nheadline: Developer\n")
		if !strings.Contains(prompt, "Test User") {
			t.Fatalf("expected profile content in prompt, got %q", prompt[:100])
		}
		if !strings.Contains(prompt, "GROUNDING") {
			t.Fatalf("expected GROUNDING section in prompt, got %q", prompt[:100])
		}
		if !strings.Contains(prompt, "CITATIONS") {
			t.Fatalf("expected CITATIONS section in prompt, got %q", prompt[:100])
		}
		if !strings.Contains(prompt, "FILTERS") {
			t.Fatalf("expected FILTERS section in prompt, got %q", prompt[:100])
		}
		if !strings.Contains(prompt, "TONE") {
			t.Fatalf("expected TONE section in prompt, got %q", prompt[:100])
		}
		if !strings.Contains(prompt, "SCOPE AND SAFETY") {
			t.Fatalf("expected SCOPE AND SAFETY section in prompt, got %q", prompt[:100])
		}
	})

	t.Run("includes profile YAML", func(t *testing.T) {
		profileYAML := "name: John Doe\nheadline: Senior Engineer\n"
		prompt := BuildSystemPrompt(profileYAML)
		if !strings.Contains(prompt, profileYAML) {
			t.Fatalf("expected profile YAML in prompt, got %q", prompt)
		}
	})

	t.Run("mentions filterable fields", func(t *testing.T) {
		prompt := BuildSystemPrompt("name: Test\n")
		if !strings.Contains(prompt, "category") {
			t.Fatalf("expected category in filter description, got %q", prompt)
		}
		if !strings.Contains(prompt, "tech_stack") {
			t.Fatalf("expected tech_stack in filter description, got %q", prompt)
		}
		if !strings.Contains(prompt, "section") {
			t.Fatalf("expected section in filter description, got %q", prompt)
		}
		if !strings.Contains(prompt, "project_name") {
			t.Fatalf("expected project_name in filter description, got %q", prompt)
		}
	})
}
