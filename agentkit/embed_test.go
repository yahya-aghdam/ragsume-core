package agentkit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewOllamaEmbedder(t *testing.T) {
	t.Run("defaults model when empty", func(t *testing.T) {
		e := NewOllamaEmbedder("http://localhost:11434", "")
		if e.Model != "nomic-embed-text" {
			t.Fatalf("got model %q, want nomic-embed-text", e.Model)
		}
	})

	t.Run("strips trailing slash from base URL", func(t *testing.T) {
		e := NewOllamaEmbedder("http://localhost:11434/", "test-model")
		if e.BaseURL != "http://localhost:11434" {
			t.Fatalf("got base URL %q, want http://localhost:11434", e.BaseURL)
		}
	})

	t.Run("sets 60s timeout", func(t *testing.T) {
		e := NewOllamaEmbedder("http://localhost:11434", "test-model")
		if e.HTTPClient.Timeout.String() != "1m0s" {
			t.Fatalf("got timeout %v, want 1m0s", e.HTTPClient.Timeout)
		}
	})
}

func TestOllamaEmbedder_Embed(t *testing.T) {
	t.Run("successful embedding", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Fatalf("expected POST, got %s", r.Method)
			}
			if r.URL.Path != "/api/embeddings" {
				t.Fatalf("expected /api/embeddings, got %s", r.URL.Path)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"embedding":[0.1,0.2,0.3]}`))
		}))
		defer server.Close()

		e := NewOllamaEmbedder(server.URL, "test-model")
		vec, err := e.Embed(context.Background(), "hello world")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(vec) != 3 || vec[0] != 0.1 {
			t.Fatalf("got %v, want [0.1 0.2 0.3]", vec)
		}
	})

	t.Run("empty embedding returns error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"embedding":[]}`))
		}))
		defer server.Close()

		e := NewOllamaEmbedder(server.URL, "test-model")
		_, err := e.Embed(context.Background(), "hello")
		if err == nil {
			t.Fatal("expected error for empty embedding")
		}
	})

	t.Run("non-200 returns error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		e := NewOllamaEmbedder(server.URL, "test-model")
		_, err := e.Embed(context.Background(), "hello")
		if err == nil {
			t.Fatal("expected error for non-200")
		}
	})
}

func TestOllamaEmbedder_Embed_HTTPClient(t *testing.T) {
	t.Run("uses custom HTTP client", func(t *testing.T) {
		e := NewOllamaEmbedder("http://localhost:11434", "test-model")
		if e.HTTPClient == nil {
			t.Fatal("expected non-nil HTTP client")
		}
	})
}
