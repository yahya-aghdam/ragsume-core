package agentkit

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
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

func TestSearchProfileScrollOnly(t *testing.T) {
	store := &mockVectorStore{
		scrollFn: func(_ context.Context, _ string, filter *Filter, _ uint64) ([]Chunk, error) {
			if filter == nil || len(filter.Must) == 0 {
				t.Fatal("expected filter")
			}
			return []Chunk{
				{ProjectName: "Proj A", Section: "problem", ChunkText: "Built APIs"},
			}, nil
		},
	}

	exec := NewToolExecutor(store, &mockEmbedder{}, nil, "projects")
	out, err := exec.Execute(context.Background(), toolSearchProfile, `{"query":"","filter":{"must":[{"field":"category","match":"backend"}]}}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result searchProfileResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(result.Chunks) != 1 || result.Chunks[0].ProjectName != "Proj A" {
		t.Fatalf("unexpected chunks: %+v", result.Chunks)
	}
}

func TestSearchProfileQueryWithFilter(t *testing.T) {
	embedded := false
	store := &mockVectorStore{
		queryFn: func(_ context.Context, _ string, _ []float32, filter *Filter, _ uint64) ([]Chunk, error) {
			if filter == nil {
				t.Fatal("expected filter")
			}
			return []Chunk{
				{ProjectName: "Proj B", Section: "outcome", ChunkText: "Shipped service", Score: float32Ptr(0.91)},
			}, nil
		},
	}
	embedder := &mockEmbedder{
		fn: func(_ context.Context, text string) ([]float32, error) {
			if text != "distributed systems" {
				t.Fatalf("unexpected query: %q", text)
			}
			embedded = true
			return []float32{0.5, 0.6}, nil
		},
	}

	exec := NewToolExecutor(store, embedder, nil, "projects")
	out, err := exec.Execute(context.Background(), toolSearchProfile, `{"query":"distributed systems","filter":{"must":[{"field":"tech_stack","match":"go"}]}}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !embedded {
		t.Fatal("expected embedding call")
	}

	var result searchProfileResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(result.Chunks) != 1 {
		t.Fatalf("unexpected chunks: %+v", result.Chunks)
	}
}

func TestSearchProfileRejectsUnknownFilterField(t *testing.T) {
	exec := NewToolExecutor(&mockVectorStore{}, &mockEmbedder{}, nil, "projects")
	_, err := exec.Execute(context.Background(), toolSearchProfile, `{"query":"","filter":{"must":[{"field":"skills","match":"go"}]}}`)
	if err == nil {
		t.Fatal("expected error for non-filterable field 'skills'")
	}
	if !strings.Contains(err.Error(), "skills") || !strings.Contains(err.Error(), "filterable") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestSearchProfileAcceptsKnownFilterFields(t *testing.T) {
	called := false
	store := &mockVectorStore{
		scrollFn: func(_ context.Context, _ string, filter *Filter, _ uint64) ([]Chunk, error) {
			called = true
			if filter == nil || len(filter.Must) != 1 || filter.Must[0].Field != "section" {
				t.Fatalf("unexpected filter: %+v", filter)
			}
			return []Chunk{{ProjectName: "Alpha", Section: "decisions"}}, nil
		},
	}
	exec := NewToolExecutor(store, &mockEmbedder{}, nil, "projects")
	out, err := exec.Execute(context.Background(), toolSearchProfile, `{"query":"","filter":{"must":[{"field":"section","match":"decisions"}]}}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("expected scroll call")
	}
	if strings.TrimSpace(out) == "" {
		t.Fatal("expected non-empty result")
	}
}

func TestMatchJobDescription(t *testing.T) {
	store := &mockVectorStore{
		queryFn: func(_ context.Context, _ string, _ []float32, _ *Filter, _ uint64) ([]Chunk, error) {
			score1 := float32(0.95)
			score2 := float32(0.80)
			return []Chunk{
				{ProjectName: "Alpha", Section: "problem", Score: &score1},
				{ProjectName: "Alpha", Section: "outcome", Score: &score2},
				{ProjectName: "Beta", Section: "decisions", Score: float32Ptr(0.70)},
			}, nil
		},
	}
	llm := &mockChatClient{
		responses: []Message{{Role: "assistant", Content: "Strong Go backend fit."}},
	}

	exec := NewToolExecutor(store, &mockEmbedder{}, llm, "projects")
	out, err := exec.Execute(context.Background(), toolMatchJobDescription, `{"jd_text":"Need Go backend engineer"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result matchJobDescriptionResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(result.Matches) != 2 {
		t.Fatalf("expected 2 project matches, got %+v", result.Matches)
	}
	if result.Pitch == "" {
		t.Fatal("expected pitch")
	}
}

func TestAgentToolLoop(t *testing.T) {
	store := &mockVectorStore{
		queryFn: func(_ context.Context, _ string, _ []float32, _ *Filter, _ uint64) ([]Chunk, error) {
			return []Chunk{{ProjectName: "Alpha", Section: "outcome", ChunkText: "Delivered platform"}}, nil
		},
	}
	llm := &mockChatClient{
		responses: []Message{
			{
				Role: "assistant",
				ToolCalls: []ToolCall{
					{
						ID:   "call_1",
						Type: "function",
						Function: FunctionCall{
							Name:      toolSearchProfile,
							Arguments: `{"query":"platform work"}`,
						},
					},
				},
			},
			{Role: "assistant", Content: "I led platform delivery on Alpha."},
		},
	}

	agent := NewAgent(llm, *NewToolExecutor(store, &mockEmbedder{}, nil, "projects"), "system")
	result, err := agent.Run(context.Background(), []Message{{Role: "user", Content: "Tell me about your platform work"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content != "I led platform delivery on Alpha." {
		t.Fatalf("unexpected content: %q", result.Content)
	}
}

func TestAgentRunStreamAfterTools(t *testing.T) {
	store := &mockVectorStore{
		queryFn: func(_ context.Context, _ string, _ []float32, _ *Filter, _ uint64) ([]Chunk, error) {
			return []Chunk{{ProjectName: "Alpha", Section: "outcome", ChunkText: "Delivered platform"}}, nil
		},
	}
	llm := &mockChatClient{
		responses: []Message{
			{
				Role: "assistant",
				ToolCalls: []ToolCall{
					{
						ID:   "call_1",
						Type: "function",
						Function: FunctionCall{
							Name:      toolSearchProfile,
							Arguments: `{"query":"platform work"}`,
						},
					},
				},
			},
			{Role: "assistant", Content: "ignored non-streaming answer"},
		},
		streamTokens: []string{"I led ", "platform ", "delivery."},
	}

	agent := NewAgent(llm, *NewToolExecutor(store, &mockEmbedder{}, nil, "projects"), "system")
	var tokens []string
	result, err := agent.RunStream(context.Background(), []Message{{Role: "user", Content: "Tell me about your platform work"}}, func(token string) error {
		tokens = append(tokens, token)
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if llm.streamCalls != 1 {
		t.Fatalf("expected 1 stream call, got %d", llm.streamCalls)
	}
	if strings.Join(tokens, "") != "I led platform delivery." {
		t.Fatalf("unexpected streamed tokens: %v", tokens)
	}
	if result.Content != "I led platform delivery." {
		t.Fatalf("unexpected content: %q", result.Content)
	}
}

func float32Ptr(v float32) *float32 {
	return &v
}
