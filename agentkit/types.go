package agentkit

import (
	"fmt"
	"net/url"
	"strings"
)

// Condition mirrors a single Qdrant filter condition.
type Condition struct {
	Field string `json:"field"`
	Match string `json:"match"`
}

// Filter mirrors Qdrant's filter schema for tool arguments.
type Filter struct {
	Must []Condition `json:"must,omitempty"`
}

// Chunk is a retrieved project chunk with citation metadata.
type Chunk struct {
	ProjectName string   `json:"project_name"`
	Section     string   `json:"section"`
	ChunkText   string   `json:"chunk_text"`
	Category    string   `json:"category,omitempty"`
	DateRange   string   `json:"date_range,omitempty"`
	TechStack   []string `json:"tech_stack,omitempty"`
	Score       *float32 `json:"score,omitempty"`
}

// PointInput is a vector point to upsert into Qdrant.
type PointInput struct {
	ID      string
	Vector  []float32
	Payload map[string]any
}

// ToQdrantFilter builds the REST API filter JSON for Qdrant.
// Returns nil when the filter is empty so the field can be omitted.
func (f *Filter) ToQdrantFilter() map[string]any {
	if f == nil || len(f.Must) == 0 {
		return nil
	}

	conditions := make([]map[string]any, 0, len(f.Must))
	for _, c := range f.Must {
		conditions = append(conditions, map[string]any{
			"key":   c.Field,
			"match": map[string]any{"value": c.Match},
		})
	}

	return map[string]any{"must": conditions}
}

// chunkFromPayload converts a REST payload (map[string]any) into a Chunk.
func chunkFromPayload(payload map[string]any, score *float32) Chunk {
	chunk := Chunk{Score: score}
	if payload == nil {
		return chunk
	}

	if v, ok := payload["project_name"]; ok {
		chunk.ProjectName = anyAsString(v)
	}
	if v, ok := payload["section"]; ok {
		chunk.Section = anyAsString(v)
	}
	if v, ok := payload["chunk_text"]; ok {
		chunk.ChunkText = anyAsString(v)
	}
	if v, ok := payload["category"]; ok {
		chunk.Category = anyAsString(v)
	}
	if v, ok := payload["date_range"]; ok {
		chunk.DateRange = anyAsString(v)
	}
	if v, ok := payload["tech_stack"]; ok {
		chunk.TechStack = anyAsStringList(v)
	}

	return chunk
}

func anyAsString(v any) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case fmt.Stringer:
		return val.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}

func anyAsStringList(v any) []string {
	if v == nil {
		return nil
	}
	list, ok := v.([]any)
	if !ok {
		// Some payloads may arrive as []string directly.
		if slist, ok := v.([]string); ok {
			return slist
		}
		return nil
	}
	out := make([]string, 0, len(list))
	for _, item := range list {
		if s := anyAsString(item); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func normalizePayload(payload map[string]any) map[string]any {
	if len(payload) == 0 {
		return payload
	}
	out := make(map[string]any, len(payload))
	for key, value := range payload {
		out[key] = normalizePayloadValue(value)
	}
	return out
}

func normalizePayloadValue(value any) any {
	switch v := value.(type) {
	case []string:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = item
		}
		return out
	default:
		return value
	}
}

// ParseQdrantURL parses a QDRANT_URL into a base URL suitable for REST calls.
// It normalizes the scheme/host/port and returns a base URL with no trailing slash.
// When no port is supplied, it defaults to 6333 (the Qdrant REST port).
func ParseQdrantURL(raw string) (baseURL string, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("qdrant url is empty")
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parse qdrant url: %w", err)
	}

	host := u.Hostname()
	if host == "" {
		host = u.Path
	}
	if host == "" {
		return "", fmt.Errorf("qdrant url has no host: %q", raw)
	}

	port := u.Port()
	if port == "" {
		if u.Scheme == "https" {
			port = "443"
		} else {
			port = "6333"
		}
	}

	scheme := u.Scheme
	if scheme == "" {
		scheme = "http"
	}

	return fmt.Sprintf("%s://%s:%s", scheme, host, port), nil
}
