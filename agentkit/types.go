package agentkit

import (
	"fmt"
	"net/url"
	"strconv"

	qdrant "github.com/qdrant/go-client/qdrant"
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

func (f *Filter) ToQdrant() *qdrant.Filter {
	if f == nil || len(f.Must) == 0 {
		return nil
	}

	conditions := make([]*qdrant.Condition, 0, len(f.Must))
	for _, c := range f.Must {
		conditions = append(conditions, qdrant.NewMatchKeyword(c.Field, c.Match))
	}

	return &qdrant.Filter{Must: conditions}
}

func chunkFromPayload(payload map[string]*qdrant.Value, score *float32) Chunk {
	chunk := Chunk{Score: score}
	if payload == nil {
		return chunk
	}

	if v, ok := payload["project_name"]; ok {
		chunk.ProjectName = valueAsString(v)
	}
	if v, ok := payload["section"]; ok {
		chunk.Section = valueAsString(v)
	}
	if v, ok := payload["chunk_text"]; ok {
		chunk.ChunkText = valueAsString(v)
	}
	if v, ok := payload["category"]; ok {
		chunk.Category = valueAsString(v)
	}
	if v, ok := payload["date_range"]; ok {
		chunk.DateRange = valueAsString(v)
	}
	if v, ok := payload["tech_stack"]; ok {
		chunk.TechStack = valueAsStringList(v)
	}

	return chunk
}

func valueAsString(v *qdrant.Value) string {
	if v == nil {
		return ""
	}
	return v.GetStringValue()
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

func valueAsStringList(v *qdrant.Value) []string {
	if v == nil {
		return nil
	}
	list := v.GetListValue()
	if list == nil {
		return nil
	}
	out := make([]string, 0, len(list.GetValues()))
	for _, item := range list.GetValues() {
		if s := item.GetStringValue(); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// ParseQdrantURL parses a QDRANT_URL into host, port, and TLS flag.
func ParseQdrantURL(raw string) (host string, port int, useTLS bool, err error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", 0, false, fmt.Errorf("parse qdrant url: %w", err)
	}

	host = u.Hostname()
	if host == "" {
		host = u.Path
	}

	port = 6334
	if p := u.Port(); p != "" {
		port, err = strconv.Atoi(p)
		if err != nil {
			return "", 0, false, fmt.Errorf("parse qdrant port: %w", err)
		}
	} else if u.Scheme == "https" {
		port = 6334
	}

	useTLS = u.Scheme == "https"
	return host, port, useTLS, nil
}
