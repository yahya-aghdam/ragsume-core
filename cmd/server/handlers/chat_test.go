package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ragsume-core/agentkit"
)

func TestNewChatHandler(t *testing.T) {
	t.Run("creates handler", func(t *testing.T) {
		agent := &agentkit.Agent{}
		h := NewChatHandler(agent)
		if h == nil {
			t.Fatal("expected non-nil handler")
		}
		if h.Agent != agent {
			t.Fatalf("expected same agent, got different")
		}
	})
}

func TestChatHandler_ServeHTTP(t *testing.T) {
	t.Run("rejects non-POST", func(t *testing.T) {
		agent := &agentkit.Agent{}
		h := NewChatHandler(agent)
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/chat", nil)

		h.ServeHTTP(w, r)

		resp := w.Result()
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Fatalf("expected 405, got %d", resp.StatusCode)
		}
	})

	t.Run("rejects empty body", func(t *testing.T) {
		agent := &agentkit.Agent{}
		h := NewChatHandler(agent)
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/chat", nil)

		h.ServeHTTP(w, r)

		resp := w.Result()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp.StatusCode)
		}
	})

	t.Run("rejects empty message", func(t *testing.T) {
		agent := &agentkit.Agent{}
		h := NewChatHandler(agent)
		w := httptest.NewRecorder()
		body := strings.NewReader(`{"message":"","history":[]}`)
		r := httptest.NewRequest(http.MethodPost, "/chat", body)
		r.Header.Set("Content-Type", "application/json")

		h.ServeHTTP(w, r)

		resp := w.Result()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp.StatusCode)
		}
	})

	t.Run("sets SSE headers", func(t *testing.T) {
		agent := &agentkit.Agent{}
		h := NewChatHandler(agent)
		w := httptest.NewRecorder()
		body := strings.NewReader(`{"message":"hello","history":[]}`)
		r := httptest.NewRequest(http.MethodPost, "/chat", body)
		r.Header.Set("Content-Type", "application/json")

		h.ServeHTTP(w, r)

		resp := w.Result()
		if resp.Header.Get("Content-Type") != "text/event-stream" {
			t.Fatalf("expected text/event-stream, got %q", resp.Header.Get("Content-Type"))
		}
		if resp.Header.Get("Cache-Control") != "no-cache" {
			t.Fatalf("expected no-cache, got %q", resp.Header.Get("Cache-Control"))
		}
	})
}

func TestChatHandler_InvalidJSON(t *testing.T) {
	agent := &agentkit.Agent{}
	h := NewChatHandler(agent)
	w := httptest.NewRecorder()
	body := strings.NewReader(`invalid json`)
	r := httptest.NewRequest(http.MethodPost, "/chat", body)
	r.Header.Set("Content-Type", "application/json")

	h.ServeHTTP(w, r)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestChatRequestMarshal(t *testing.T) {
	t.Run("marshals correctly", func(t *testing.T) {
		req := chatRequest{
			Message: "hello",
			History: []agentkit.Message{
				{Role: "user", Content: "previous"},
			},
		}
		data, err := json.Marshal(req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(string(data), "hello") {
			t.Fatalf("expected hello in JSON, got %s", data)
		}
	})
}
