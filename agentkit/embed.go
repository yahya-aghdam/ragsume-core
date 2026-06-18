package agentkit

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Embedder generates vector embeddings from text.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// OllamaEmbedder calls a local Ollama server for embeddings.
type OllamaEmbedder struct {
	BaseURL    string
	Model      string
	HTTPClient *http.Client
}

// NewOllamaEmbedder creates an embedder with sensible defaults.
func NewOllamaEmbedder(baseURL, model string) *OllamaEmbedder {
	if model == "" {
		model = "nomic-embed-text"
	}
	return &OllamaEmbedder{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Model:   model,
		HTTPClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

type ollamaEmbedRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type ollamaEmbedResponse struct {
	Embedding []float32 `json:"embedding"`
}

func (e *OllamaEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	body, err := json.Marshal(ollamaEmbedRequest{
		Model:  e.Model,
		Prompt: text,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal ollama embed request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.BaseURL+"/api/embeddings", strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("create ollama embed request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama embed request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama embed returned status %d", resp.StatusCode)
	}

	var parsed ollamaEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("decode ollama embed response: %w", err)
	}
	if len(parsed.Embedding) == 0 {
		return nil, fmt.Errorf("ollama returned empty embedding")
	}

	return parsed.Embedding, nil
}
