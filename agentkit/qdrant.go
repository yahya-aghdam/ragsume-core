package agentkit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// VectorStore abstracts Qdrant operations used by tools and ingest.
type VectorStore interface {
	EnsureCollection(ctx context.Context, name string, vectorSize uint64) error
	EnsurePayloadIndexes(ctx context.Context, collection string, fields []string) error
	Upsert(ctx context.Context, collection string, points []PointInput) error
	Scroll(ctx context.Context, collection string, filter *Filter, limit uint64) ([]Chunk, error)
	Query(ctx context.Context, collection string, vector []float32, filter *Filter, limit uint64) ([]Chunk, error)
	Close() error
}

// QdrantClient talks to Qdrant over its REST API (default port 6333).
type QdrantClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewQdrantClient connects to Qdrant using a URL and optional API key.
func NewQdrantClient(rawURL, apiKey string) (*QdrantClient, error) {
	baseURL, err := ParseQdrantURL(rawURL)
	if err != nil {
		return nil, err
	}

	return &QdrantClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}, nil
}

func (c *QdrantClient) Close() error {
	return nil
}

// do performs an HTTP request against the Qdrant REST API, attaching the API
// key header when configured, and returns the decoded JSON body.
func (c *QdrantClient) do(ctx context.Context, method, path string, body any) (map[string]any, error) {
	var reqBody io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.apiKey != "" {
		req.Header.Set("api-key", c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("qdrant request %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read qdrant response: %w", err)
	}

	// Treat non-2xx as an error. Include a snippet of the body for debugging.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet := string(raw)
		if len(snippet) > 512 {
			snippet = snippet[:512]
		}
		return nil, fmt.Errorf("qdrant %s %s returned status %d: %s", method, path, resp.StatusCode, snippet)
	}

	var parsed map[string]any
	if len(raw) == 0 {
		return parsed, nil
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("decode qdrant response: %w", err)
	}
	return parsed, nil
}

// EnsureCollection creates the collection if it does not already exist.
func (c *QdrantClient) EnsureCollection(ctx context.Context, name string, vectorSize uint64) error {
	// GET /collections/{name} -> 200 if exists, 404 otherwise.
	_, err := c.do(ctx, http.MethodGet, "/collections/"+name, nil)
	if err == nil {
		return nil
	}
	// A 404 surfaces as a status error; only proceed to create on that case.
	// Any other transport error should bubble up.
	if !isNotFound(err) {
		return fmt.Errorf("check collection exists: %w", err)
	}

	body := map[string]any{
		"vectors": map[string]any{
			"size":     vectorSize,
			"distance": "Cosine",
		},
	}
	if _, err := c.do(ctx, http.MethodPut, "/collections/"+name, body); err != nil {
		return fmt.Errorf("create collection: %w", err)
	}
	return nil
}

// EnsurePayloadIndexes creates keyword field indexes for the given payload fields.
func (c *QdrantClient) EnsurePayloadIndexes(ctx context.Context, collection string, fields []string) error {
	for _, field := range fields {
		body := map[string]any{
			"field_name": field,
			"field_schema": map[string]any{
				"type": "keyword",
			},
			"wait": true,
		}
		if _, err := c.do(ctx, http.MethodPut, "/collections/"+collection+"/index", body); err != nil {
			return fmt.Errorf("create payload index on %q: %w", field, err)
		}
	}
	return nil
}

// Upsert inserts or updates points in the collection.
func (c *QdrantClient) Upsert(ctx context.Context, collection string, points []PointInput) error {
	if len(points) == 0 {
		return nil
	}

	restPoints := make([]map[string]any, 0, len(points))
	for _, p := range points {
		restPoints = append(restPoints, map[string]any{
			"id":      p.ID,
			"vector":  p.Vector,
			"payload": normalizePayload(p.Payload),
		})
	}

	body := map[string]any{
		"points": restPoints,
		"wait":   true,
	}
	if _, err := c.do(ctx, http.MethodPut, "/collections/"+collection+"/points", body); err != nil {
		return fmt.Errorf("upsert points: %w", err)
	}
	return nil
}

// Scroll returns points matching the filter (metadata-only listing).
func (c *QdrantClient) Scroll(ctx context.Context, collection string, filter *Filter, limit uint64) ([]Chunk, error) {
	if limit == 0 {
		limit = 20
	}

	body := map[string]any{
		"limit":        limit,
		"with_payload": true,
		"with_vector":  false,
	}
	if qf := filter.ToQdrantFilter(); qf != nil {
		body["filter"] = qf
	}

	parsed, err := c.do(ctx, http.MethodPost, "/collections/"+collection+"/points/scroll", body)
	if err != nil {
		return nil, fmt.Errorf("scroll points: %w", err)
	}

	return chunksFromScrollResult(parsed), nil
}

// Query performs a nearest-neighbor search against the given vector.
func (c *QdrantClient) Query(ctx context.Context, collection string, vector []float32, filter *Filter, limit uint64) ([]Chunk, error) {
	if limit == 0 {
		limit = 10
	}

	body := map[string]any{
		"query":        vector,
		"limit":        limit,
		"with_payload": true,
		"with_vector":  false,
	}
	if qf := filter.ToQdrantFilter(); qf != nil {
		body["filter"] = qf
	}

	parsed, err := c.do(ctx, http.MethodPost, "/collections/"+collection+"/points/query", body)
	if err != nil {
		return nil, fmt.Errorf("query points: %w", err)
	}

	return chunksFromQueryResult(parsed), nil
}

// chunksFromScrollResult extracts chunks from a /points/scroll response.
func chunksFromScrollResult(parsed map[string]any) []Chunk {
	points := extractPoints(parsed)
	out := make([]Chunk, 0, len(points))
	for _, p := range points {
		out = append(out, chunkFromPayload(extractPayload(p), nil))
	}
	return out
}

// chunksFromQueryResult extracts chunks (with scores) from a /points/query response.
func chunksFromQueryResult(parsed map[string]any) []Chunk {
	points := extractPoints(parsed)
	out := make([]Chunk, 0, len(points))
	for _, p := range points {
		score := extractScore(p)
		out = append(out, chunkFromPayload(extractPayload(p), score))
	}
	return out
}

// extractPoints pulls the "points" array out of a REST response object.
func extractPoints(parsed map[string]any) []map[string]any {
	if parsed == nil {
		return nil
	}
	if result, ok := parsed["result"].(map[string]any); ok {
		if points, ok := result["points"].([]any); ok {
			return toPointMaps(points)
		}
		// scroll responses nest points directly under result.
		if points, ok := result["points"].([]any); ok {
			return toPointMaps(points)
		}
	}
	// Some endpoints return points at the top level.
	if points, ok := parsed["points"].([]any); ok {
		return toPointMaps(points)
	}
	return nil
}

func toPointMaps(points []any) []map[string]any {
	out := make([]map[string]any, 0, len(points))
	for _, p := range points {
		if m, ok := p.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func extractPayload(point map[string]any) map[string]any {
	if point == nil {
		return nil
	}
	if payload, ok := point["payload"].(map[string]any); ok {
		return payload
	}
	return nil
}

func extractScore(point map[string]any) *float32 {
	if point == nil {
		return nil
	}
	score, ok := point["score"]
	if !ok {
		return nil
	}
	switch v := score.(type) {
	case float64:
		s := float32(v)
		return &s
	case float32:
		return &v
	case int:
		s := float32(v)
		return &s
	case int64:
		s := float32(v)
		return &s
	default:
		return nil
	}
}

// isNotFound reports whether an error originated from a 404 response.
func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "status 404")
}
