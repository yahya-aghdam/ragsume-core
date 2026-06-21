package agentkit

import (
	"testing"
)

func TestFilterToQdrantFilter(t *testing.T) {
	t.Run("nil filter returns nil", func(t *testing.T) {
		var f *Filter
		if got := f.ToQdrantFilter(); got != nil {
			t.Fatalf("expected nil, got %v", got)
		}
	})

	t.Run("empty must returns nil", func(t *testing.T) {
		f := &Filter{}
		if got := f.ToQdrantFilter(); got != nil {
			t.Fatalf("expected nil, got %v", got)
		}
	})

	t.Run("single condition", func(t *testing.T) {
		f := &Filter{Must: []Condition{{Field: "category", Match: "backend"}}}
		got := f.ToQdrantFilter()
		if got == nil {
			t.Fatal("expected non-nil filter")
		}
		must, ok := got["must"].([]map[string]any)
		if !ok || len(must) != 1 {
			t.Fatalf("expected 1 must condition, got %+v", got)
		}
		if must[0]["key"] != "category" {
			t.Fatalf("expected key 'category', got %v", must[0]["key"])
		}
		if must[0]["match"].(map[string]any)["value"] != "backend" {
			t.Fatalf("expected match value 'backend', got %v", must[0]["match"])
		}
	})

	t.Run("multiple conditions", func(t *testing.T) {
		f := &Filter{Must: []Condition{
			{Field: "tech_stack", Match: "go"},
			{Field: "section", Match: "decisions"},
		}}
		got := f.ToQdrantFilter()
		if got == nil {
			t.Fatal("expected non-nil filter")
		}
		must, ok := got["must"].([]map[string]any)
		if !ok || len(must) != 2 {
			t.Fatalf("expected 2 must conditions, got %d", len(must))
		}
	})
}

func TestChunkFromPayload(t *testing.T) {
	t.Run("nil payload returns empty chunk", func(t *testing.T) {
		chunk := chunkFromPayload(nil, nil)
		if chunk.ProjectName != "" || chunk.Section != "" {
			t.Fatalf("expected empty chunk, got %+v", chunk)
		}
	})

	t.Run("populates all fields", func(t *testing.T) {
		payload := map[string]any{
			"project_name": "Alpha",
			"section":      "outcome",
			"chunk_text":   "Delivered platform",
			"category":     "backend",
			"date_range":   "2023-2024",
			"tech_stack":   []any{"go", "grpc"},
		}
		score := float32Ptr(0.95)
		chunk := chunkFromPayload(payload, score)
		if chunk.ProjectName != "Alpha" {
			t.Fatalf("got project_name %q, want Alpha", chunk.ProjectName)
		}
		if chunk.Section != "outcome" {
			t.Fatalf("got section %q, want outcome", chunk.Section)
		}
		if chunk.ChunkText != "Delivered platform" {
			t.Fatalf("got chunk_text %q, want 'Delivered platform'", chunk.ChunkText)
		}
		if chunk.Category != "backend" {
			t.Fatalf("got category %q, want backend", chunk.Category)
		}
		if chunk.DateRange != "2023-2024" {
			t.Fatalf("got date_range %q, want 2023-2024", chunk.DateRange)
		}
		if len(chunk.TechStack) != 2 || chunk.TechStack[0] != "go" {
			t.Fatalf("got tech_stack %v, want [go grpc]", chunk.TechStack)
		}
		if *chunk.Score != 0.95 {
			t.Fatalf("got score %v, want 0.95", *chunk.Score)
		}
	})

	t.Run("missing fields remain empty", func(t *testing.T) {
		payload := map[string]any{"project_name": "Beta"}
		chunk := chunkFromPayload(payload, nil)
		if chunk.ProjectName != "Beta" {
			t.Fatalf("got project_name %q, want Beta", chunk.ProjectName)
		}
		if chunk.Section != "" {
			t.Fatalf("expected empty section, got %q", chunk.Section)
		}
	})
}

func TestAnyAsString(t *testing.T) {
	t.Run("nil returns empty", func(t *testing.T) {
		if got := anyAsString(nil); got != "" {
			t.Fatalf("got %q, want empty", got)
		}
	})

	t.Run("string value", func(t *testing.T) {
		if got := anyAsString("hello"); got != "hello" {
			t.Fatalf("got %q, want hello", got)
		}
	})

	t.Run("non-string value", func(t *testing.T) {
		if got := anyAsString(42); got != "42" {
			t.Fatalf("got %q, want 42", got)
		}
	})
}

func TestAnyAsStringList(t *testing.T) {
	t.Run("nil returns nil", func(t *testing.T) {
		if got := anyAsStringList(nil); got != nil {
			t.Fatalf("expected nil, got %v", got)
		}
	})

	t.Run("[]any slice", func(t *testing.T) {
		input := []any{"go", "python", 42}
		got := anyAsStringList(input)
		if len(got) != 3 || got[0] != "go" {
			t.Fatalf("got %v, want [go python 42]", got)
		}
	})

	t.Run("[]string slice", func(t *testing.T) {
		input := []string{"go", "python"}
		got := anyAsStringList(input)
		if len(got) != 2 || got[0] != "go" {
			t.Fatalf("got %v, want [go python]", got)
		}
	})
}

func TestParseQdrantURL(t *testing.T) {
	t.Run("empty string", func(t *testing.T) {
		_, err := ParseQdrantURL("")
		if err == nil {
			t.Fatal("expected error for empty URL")
		}
	})

	t.Run("valid URL with port", func(t *testing.T) {
		got, err := ParseQdrantURL("http://localhost:6333")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "http://localhost:6333" {
			t.Fatalf("got %q, want http://localhost:6333", got)
		}
	})

	t.Run("defaults to port 6333", func(t *testing.T) {
		got, err := ParseQdrantURL("http://localhost")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "http://localhost:6333" {
			t.Fatalf("got %q, want http://localhost:6333", got)
		}
	})

	t.Run("https defaults to port 443", func(t *testing.T) {
		got, err := ParseQdrantURL("https://qdrant.example.com")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "https://qdrant.example.com:443" {
			t.Fatalf("got %q, want https://qdrant.example.com:443", got)
		}
	})

	t.Run("trims whitespace", func(t *testing.T) {
		got, err := ParseQdrantURL("  http://localhost:6333  ")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "http://localhost:6333" {
			t.Fatalf("got %q, want http://localhost:6333", got)
		}
	})
}

func TestNormalizePayload(t *testing.T) {
	t.Run("empty payload", func(t *testing.T) {
		got := normalizePayload(nil)
		if got != nil {
			t.Fatalf("expected nil, got %v", got)
		}
	})

	t.Run("converts []string to []any", func(t *testing.T) {
		payload := map[string]any{"tech_stack": []string{"go", "python"}}
		got := normalizePayload(payload)
		techStack, ok := got["tech_stack"].([]any)
		if !ok {
			t.Fatalf("expected []any, got %T", got["tech_stack"])
		}
		if len(techStack) != 2 {
			t.Fatalf("expected 2 items, got %d", len(techStack))
		}
	})
}
