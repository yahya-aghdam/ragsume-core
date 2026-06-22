package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"ragsume-core/config"
)

func TestCORS(t *testing.T) {
	// Set up the global config so CORS middleware has an origin to use.
	config.C.AllowedOrigin = "http://localhost:3000"

	t.Run("sets CORS headers", func(t *testing.T) {
		handler := CORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		handler.ServeHTTP(w, r)

		resp := w.Result()
		if resp.Header.Get("Access-Control-Allow-Origin") == "" {
			t.Fatal("expected Access-Control-Allow-Origin header")
		}
		if resp.Header.Get("Access-Control-Allow-Methods") == "" {
			t.Fatal("expected Access-Control-Allow-Methods header")
		}
		if resp.Header.Get("Access-Control-Allow-Headers") == "" {
			t.Fatal("expected Access-Control-Allow-Headers header")
		}
	})

	t.Run("handles OPTIONS preflight", func(t *testing.T) {
		handler := CORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodOptions, "/", nil)
		handler.ServeHTTP(w, r)

		resp := w.Result()
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("expected 204 for OPTIONS, got %d", resp.StatusCode)
		}
	})
}
