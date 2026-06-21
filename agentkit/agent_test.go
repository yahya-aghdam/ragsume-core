package agentkit

import (
	"context"
	"strings"
)

type mockVectorStore struct {
	scrollFn func(ctx context.Context, collection string, filter *Filter, limit uint64) ([]Chunk, error)
	queryFn  func(ctx context.Context, collection string, vector []float32, filter *Filter, limit uint64) ([]Chunk, error)
}

func (m *mockVectorStore) EnsureCollection(context.Context, string, uint64) error { return nil }
func (m *mockVectorStore) EnsurePayloadIndexes(context.Context, string, []string) error {
	return nil
}
func (m *mockVectorStore) Upsert(context.Context, string, []PointInput) error { return nil }
func (m *mockVectorStore) Close() error                                       { return nil }

func (m *mockVectorStore) Scroll(ctx context.Context, collection string, filter *Filter, limit uint64) ([]Chunk, error) {
	if m.scrollFn != nil {
		return m.scrollFn(ctx, collection, filter, limit)
	}
	return nil, nil
}

func (m *mockVectorStore) Query(ctx context.Context, collection string, vector []float32, filter *Filter, limit uint64) ([]Chunk, error) {
	if m.queryFn != nil {
		return m.queryFn(ctx, collection, vector, filter, limit)
	}
	return nil, nil
}

type mockEmbedder struct {
	fn func(ctx context.Context, text string) ([]float32, error)
}

func (m *mockEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	if m.fn != nil {
		return m.fn(ctx, text)
	}
	return []float32{0.1, 0.2}, nil
}

type mockChatClient struct {
	responses    []Message
	calls        int
	streamCalls  int
	streamTokens []string
}

func (m *mockChatClient) Complete(ctx context.Context, req ChatCompletionRequest) (Message, error) {
	if m.calls >= len(m.responses) {
		return Message{Role: "assistant", Content: "done"}, nil
	}
	resp := m.responses[m.calls]
	m.calls++
	return resp, nil
}

func float32Ptr(v float32) *float32 {
	return &v
}

func (m *mockChatClient) CompleteStream(ctx context.Context, req ChatCompletionRequest, onToken func(string) error) (Message, error) {
	m.streamCalls++
	tokens := m.streamTokens
	if len(tokens) == 0 {
		tokens = []string{"streamed answer"}
	}
	var content strings.Builder
	for _, token := range tokens {
		content.WriteString(token)
		if onToken != nil {
			if err := onToken(token); err != nil {
				return Message{}, err
			}
		}
	}
	return Message{Role: "assistant", Content: content.String()}, nil
}
