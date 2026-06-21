package agentkit

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewQdrantClient(t *testing.T) {
	t.Run("parses URL", func(t *testing.T) {
		client, err := NewQdrantClient("http://localhost:6333", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if client == nil {
			t.Fatal("expected non-nil client")
		}
	})

	t.Run("fails on empty URL", func(t *testing.T) {
		_, err := NewQdrantClient("", "")
		if err == nil {
			t.Fatal("expected error for empty URL")
		}
	})
}

func TestQdrantClient_Close(t *testing.T) {
	client, err := NewQdrantClient("http://localhost:6333", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestQdrantClient_EnsureCollection(t *testing.T) {
	t.Run("creates collection when not found", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			if r.Method == http.MethodPut {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"result":true}`))
				return
			}
			t.Fatalf("unexpected method: %s", r.Method)
		}))
		defer server.Close()

		client, err := NewQdrantClient(server.URL, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		client.httpClient = server.Client()

		err = client.EnsureCollection(context.Background(), "test", 768)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("returns nil when collection exists", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"result":true}`))
		}))
		defer server.Close()

		client, err := NewQdrantClient(server.URL, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		client.httpClient = server.Client()

		err = client.EnsureCollection(context.Background(), "existing", 768)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestQdrantClient_Upsert(t *testing.T) {
	t.Run("returns nil for empty points", func(t *testing.T) {
		client, err := NewQdrantClient("http://localhost:6333", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		err = client.Upsert(context.Background(), "test", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("upserts points", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"result":true}`))
		}))
		defer server.Close()

		client, err := NewQdrantClient(server.URL, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		client.httpClient = server.Client()

		points := []PointInput{
			{ID: "1", Vector: []float32{0.1, 0.2}, Payload: map[string]any{"project_name": "Alpha"}},
		}
		err = client.Upsert(context.Background(), "test", points)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestQdrantClient_Scroll(t *testing.T) {
	t.Run("scrolls with filter", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"result":{"points":[{"id":"1","payload":{"project_name":"Alpha","section":"outcome"}}]}}`))
		}))
		defer server.Close()

		client, err := NewQdrantClient(server.URL, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		client.httpClient = server.Client()

		chunks, err := client.Scroll(context.Background(), "test", &Filter{Must: []Condition{{Field: "section", Match: "outcome"}}}, 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(chunks) != 1 || chunks[0].ProjectName != "Alpha" {
			t.Fatalf("got %+v, want 1 chunk with Alpha", chunks)
		}
	})
}

func TestQdrantClient_Query(t *testing.T) {
	t.Run("queries with vector", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"result":{"points":[{"id":"1","payload":{"project_name":"Beta","section":"problem"},"score":0.95}]}}`))
		}))
		defer server.Close()

		client, err := NewQdrantClient(server.URL, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		client.httpClient = server.Client()

		chunks, err := client.Query(context.Background(), "test", []float32{0.1, 0.2}, nil, 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(chunks) != 1 || chunks[0].ProjectName != "Beta" {
			t.Fatalf("got %+v, want 1 chunk with Beta", chunks)
		}
	})
}

func TestIsNotFound(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		if isNotFound(nil) {
			t.Fatal("expected false for nil")
		}
	})

	t.Run("404 error", func(t *testing.T) {
		if !isNotFound(NewQdrantError("status 404")) {
			t.Fatal("expected true for 404")
		}
	})

	t.Run("non-404 error", func(t *testing.T) {
		if isNotFound(NewQdrantError("status 500")) {
			t.Fatal("expected false for 500")
		}
	})
}

// NewQdrantError creates a simple error for testing
func NewQdrantError(msg string) error {
	return &qdrantError{msg: msg}
}

type qdrantError struct {
	msg string
}

func (e *qdrantError) Error() string {
	return e.msg
}

func TestSanitizeQdrantRequestBody(t *testing.T) {
	t.Run("nil body", func(t *testing.T) {
		got := sanitizeQdrantRequestBody(nil, nil)
		if got != "" {
			t.Fatalf("got %q, want empty", got)
		}
	})

	t.Run("sanitizes vector", func(t *testing.T) {
		body := map[string]any{
			"query": []float32{0.1, 0.2, 0.3},
			"limit": 10,
		}
		encoded, _ := json.Marshal(body)
		got := sanitizeQdrantRequestBody(body, encoded)
		if !strings.Contains(got, "[vector:3 dims]") {
			t.Fatalf("expected vector summary, got %q", got)
		}
	})
}

func TestVectorSummary(t *testing.T) {
	t.Run("[]float32", func(t *testing.T) {
		got := vectorSummary([]float32{0.1, 0.2})
		if got != "[vector:2 dims]" {
			t.Fatalf("got %q, want [vector:2 dims]", got)
		}
	})

	t.Run("[]any", func(t *testing.T) {
		got := vectorSummary([]any{0.1, 0.2})
		if got != "[vector:2 dims]" {
			t.Fatalf("got %q, want [vector:2 dims]", got)
		}
	})
}

func TestTruncateLogBody(t *testing.T) {
	t.Run("short body", func(t *testing.T) {
		got := truncateLogBody([]byte("hello"))
		if got != "hello" {
			t.Fatalf("got %q, want hello", got)
		}
	})

	t.Run("long body truncated", func(t *testing.T) {
		long := make([]byte, 5000)
		for i := range long {
			long[i] = 'a'
		}
		got := truncateLogBody(long)
		if len(got) != 4096+len("...(truncated)") {
			t.Fatalf("expected truncated, got %d chars", len(got))
		}
	})
}

func TestExtractPoints(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		if got := extractPoints(nil); got != nil {
			t.Fatalf("expected nil, got %v", got)
		}
	})

	t.Run("top-level points", func(t *testing.T) {
		parsed := map[string]any{
			"points": []any{
				map[string]any{"payload": map[string]any{"project_name": "Alpha"}},
			},
		}
		got := extractPoints(parsed)
		if len(got) != 1 {
			t.Fatalf("expected 1 point, got %d", len(got))
		}
	})
}

func TestExtractPayload(t *testing.T) {
	t.Run("nil point", func(t *testing.T) {
		if got := extractPayload(nil); got != nil {
			t.Fatalf("expected nil, got %v", got)
		}
	})

	t.Run("with payload", func(t *testing.T) {
		point := map[string]any{"payload": map[string]any{"project_name": "Alpha"}}
		got := extractPayload(point)
		if got["project_name"] != "Alpha" {
			t.Fatalf("expected Alpha, got %v", got["project_name"])
		}
	})
}

func TestExtractScore(t *testing.T) {
	t.Run("nil point", func(t *testing.T) {
		if got := extractScore(nil); got != nil {
			t.Fatalf("expected nil, got %v", got)
		}
	})

	t.Run("float64 score", func(t *testing.T) {
		point := map[string]any{"score": float64(0.95)}
		got := extractScore(point)
		if got == nil || *got != 0.95 {
			t.Fatalf("expected 0.95, got %v", got)
		}
	})

	t.Run("missing score", func(t *testing.T) {
		point := map[string]any{}
		if got := extractScore(point); got != nil {
			t.Fatalf("expected nil, got %v", got)
		}
	})
}

func TestChunksFromScrollResult(t *testing.T) {
	parsed := map[string]any{
		"result": map[string]any{
			"points": []any{
				map[string]any{
					"payload": map[string]any{
						"project_name": "Alpha",
						"section":      "outcome",
					},
				},
			},
		},
	}
	chunks := chunksFromScrollResult(parsed)
	if len(chunks) != 1 || chunks[0].ProjectName != "Alpha" {
		t.Fatalf("got %+v, want 1 chunk with Alpha", chunks)
	}
}

func TestChunksFromQueryResult(t *testing.T) {
	parsed := map[string]any{
		"result": map[string]any{
			"points": []any{
				map[string]any{
					"payload": map[string]any{
						"project_name": "Beta",
						"section":      "problem",
					},
					"score": float64(0.95),
				},
			},
		},
	}
	chunks := chunksFromQueryResult(parsed)
	if len(chunks) != 1 || chunks[0].ProjectName != "Beta" {
		t.Fatalf("got %+v, want 1 chunk with Beta", chunks)
	}
	if chunks[0].Score == nil || *chunks[0].Score != 0.95 {
		t.Fatalf("expected score 0.95, got %v", chunks[0].Score)
	}
}
