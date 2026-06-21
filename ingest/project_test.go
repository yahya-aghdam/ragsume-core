package main

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestParseProject(t *testing.T) {
	t.Run("parses valid YAML", func(t *testing.T) {
		data := []byte("project_name: Alpha\ncategory: backend\ndate_range: 2023-2024\ntech_stack:\n  - Go\n  - gRPC\nproblem: Built APIs\ndecisions: Chose Go\ntradeoffs: None\noutcome: Shipped\n")
		project, err := parseProject(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if project.ProjectName != "Alpha" {
			t.Fatalf("got project_name %q, want Alpha", project.ProjectName)
		}
		if project.Category != "backend" {
			t.Fatalf("got category %q, want backend", project.Category)
		}
		if len(project.TechStack) != 2 || project.TechStack[0] != "Go" {
			t.Fatalf("got tech_stack %v, want [Go gRPC]", project.TechStack)
		}
	})

	t.Run("fails on empty project name", func(t *testing.T) {
		data := []byte("project_name: \"\"\n")
		_, err := parseProject(data)
		if err == nil {
			t.Fatal("expected error for empty project name")
		}
		if !strings.Contains(err.Error(), "project_name is required") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("fails on invalid YAML", func(t *testing.T) {
		data := []byte("invalid: [yaml: broken")
		_, err := parseProject(data)
		if err == nil {
			t.Fatal("expected error for invalid YAML")
		}
	})
}

func TestChunkProject(t *testing.T) {
	t.Run("creates chunks for non-empty sections", func(t *testing.T) {
		p := Project{
			ProjectName: "Alpha",
			Problem:     "Built APIs",
			Decisions:   "Chose Go",
			Outcome:     "Shipped",
		}
		chunks := chunkProject(p)
		if len(chunks) != 3 {
			t.Fatalf("expected 3 chunks, got %d", len(chunks))
		}
		if chunks[0].Section != "problem" {
			t.Fatalf("expected first section 'problem', got %q", chunks[0].Section)
		}
		if chunks[1].Section != "decisions" {
			t.Fatalf("expected second section 'decisions', got %q", chunks[1].Section)
		}
		if chunks[2].Section != "outcome" {
			t.Fatalf("expected third section 'outcome', got %q", chunks[2].Section)
		}
	})

	t.Run("skips empty sections", func(t *testing.T) {
		p := Project{
			ProjectName: "Beta",
			Problem:     "  ",
			Decisions:   "",
			Outcome:     "Shipped",
		}
		chunks := chunkProject(p)
		if len(chunks) != 1 {
			t.Fatalf("expected 1 chunk, got %d", len(chunks))
		}
		if chunks[0].Section != "outcome" {
			t.Fatalf("expected 'outcome' section, got %q", chunks[0].Section)
		}
	})

	t.Run("returns empty for all empty", func(t *testing.T) {
		p := Project{ProjectName: "Gamma"}
		chunks := chunkProject(p)
		if len(chunks) != 0 {
			t.Fatalf("expected 0 chunks, got %d", len(chunks))
		}
	})
}

func TestNormalizeTechStack(t *testing.T) {
	t.Run("lowercases and trims", func(t *testing.T) {
		got := normalizeTechStack([]string{"  Go  ", "gRPC", "  "})
		if len(got) != 2 {
			t.Fatalf("expected 2 items, got %d", len(got))
		}
		if got[0] != "go" {
			t.Fatalf("expected 'go', got %q", got[0])
		}
		if got[1] != "grpc" {
			t.Fatalf("expected 'grpc', got %q", got[1])
		}
	})

	t.Run("filters empty strings", func(t *testing.T) {
		got := normalizeTechStack([]string{"", "  ", "go"})
		if len(got) != 1 {
			t.Fatalf("expected 1 item, got %d", len(got))
		}
	})
}

func TestPointID(t *testing.T) {
	t.Run("generates unique IDs", func(t *testing.T) {
		id1 := pointID("Alpha", "problem")
		id2 := pointID("Alpha", "outcome")
		if id1 == id2 {
			t.Fatalf("expected different IDs for different sections")
		}
	})

	t.Run("same section same project same ID", func(t *testing.T) {
		id1 := pointID("Alpha", "problem")
		id2 := pointID("Alpha", "problem")
		if id1 != id2 {
			t.Fatalf("expected same ID for same inputs")
		}
	})
}

func TestNormalizeTechStackEmpty(t *testing.T) {
	got := normalizeTechStack(nil)
	if got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestParseProjectYAML(t *testing.T) {
	t.Run("uses yaml.v3 unmarshaler", func(t *testing.T) {
		data := []byte("project_name: Test\ncategory: backend\n")
		var p Project
		if err := yaml.Unmarshal(data, &p); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p.ProjectName != "Test" {
			t.Fatalf("got %q, want Test", p.ProjectName)
		}
	})
}
