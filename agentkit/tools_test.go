package agentkit

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestNewToolExecutor(t *testing.T) {
	t.Run("defaults collection when empty", func(t *testing.T) {
		exec := NewToolExecutor(&mockVectorStore{}, &mockEmbedder{}, nil, "")
		if exec.Collection != "projects" {
			t.Fatalf("got collection %q, want projects", exec.Collection)
		}
	})

	t.Run("uses provided collection", func(t *testing.T) {
		exec := NewToolExecutor(&mockVectorStore{}, &mockEmbedder{}, nil, "custom")
		if exec.Collection != "custom" {
			t.Fatalf("got collection %q, want custom", exec.Collection)
		}
	})
}

func TestToolExecutor_Execute(t *testing.T) {
	t.Run("unknown tool returns error", func(t *testing.T) {
		exec := NewToolExecutor(&mockVectorStore{}, &mockEmbedder{}, nil, "projects")
		_, err := exec.Execute(context.Background(), "unknown_tool", "{}")
		if err == nil {
			t.Fatal("expected error for unknown tool")
		}
		if !strings.Contains(err.Error(), "unknown_tool") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("search_profile with empty query and no filter", func(t *testing.T) {
		exec := NewToolExecutor(&mockVectorStore{}, &mockEmbedder{}, nil, "projects")
		_, err := exec.Execute(context.Background(), toolSearchProfile, `{"query":"","filter":null}`)
		if err == nil {
			t.Fatal("expected error for empty query and no filter")
		}
	})

	t.Run("search_profile with query only", func(t *testing.T) {
		embedded := false
		store := &mockVectorStore{
			queryFn: func(_ context.Context, _ string, _ []float32, _ *Filter, _ uint64) ([]Chunk, error) {
				return []Chunk{{ProjectName: "Alpha", Section: "outcome", ChunkText: "Delivered"}}, nil
			},
		}
		embedder := &mockEmbedder{
			fn: func(_ context.Context, text string) ([]float32, error) {
				embedded = true
				return []float32{0.1, 0.2}, nil
			},
		}
		exec := NewToolExecutor(store, embedder, nil, "projects")
		out, err := exec.Execute(context.Background(), toolSearchProfile, `{"query":"platform work"}`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !embedded {
			t.Fatal("expected embedding call")
		}
		var result searchProfileResult
		if err := json.Unmarshal([]byte(out), &result); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if len(result.Chunks) != 1 {
			t.Fatalf("expected 1 chunk, got %d", len(result.Chunks))
		}
	})

	t.Run("list_projects", func(t *testing.T) {
		store := &mockVectorStore{
			scrollFn: func(_ context.Context, _ string, _ *Filter, _ uint64) ([]Chunk, error) {
				return []Chunk{
					{ProjectName: "Alpha"},
					{ProjectName: "Beta"},
					{ProjectName: "Alpha"}, // duplicate
				}, nil
			},
		}
		exec := NewToolExecutor(store, &mockEmbedder{}, nil, "projects")
		out, err := exec.Execute(context.Background(), toolListProjects, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var result map[string]any
		if err := json.Unmarshal([]byte(out), &result); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		projects, ok := result["projects"].([]any)
		if !ok {
			t.Fatalf("expected projects array, got %T", result["projects"])
		}
		if len(projects) != 2 {
			t.Fatalf("expected 2 unique projects, got %d", len(projects))
		}
	})
}

func TestDefaultTools(t *testing.T) {
	t.Run("returns copy of defaults", func(t *testing.T) {
		tools := DefaultTools()
		if len(tools) != 3 {
			t.Fatalf("expected 3 tools, got %d", len(tools))
		}
		if tools[0].Function.Name != toolSearchProfile {
			t.Fatalf("expected first tool %q, got %q", toolSearchProfile, tools[0].Function.Name)
		}
		if tools[1].Function.Name != toolMatchJobDescription {
			t.Fatalf("expected second tool %q, got %q", toolMatchJobDescription, tools[1].Function.Name)
		}
		if tools[2].Function.Name != toolListProjects {
			t.Fatalf("expected third tool %q, got %q", toolListProjects, tools[2].Function.Name)
		}
	})
}

func TestIsFilterableField(t *testing.T) {
	t.Run("known fields", func(t *testing.T) {
		known := []string{"category", "tech_stack", "section", "project_name"}
		for _, f := range known {
			if !isFilterableField(f) {
				t.Fatalf("expected %q to be filterable", f)
			}
		}
	})

	t.Run("unknown fields", func(t *testing.T) {
		unknown := []string{"skills", "experience", "unknown"}
		for _, f := range unknown {
			if isFilterableField(f) {
				t.Fatalf("expected %q to not be filterable", f)
			}
		}
	})
}

func TestAggregateMatches(t *testing.T) {
	t.Run("empty chunks", func(t *testing.T) {
		matches := aggregateMatches(nil)
		if len(matches) != 0 {
			t.Fatalf("expected 0 matches, got %d", len(matches))
		}
	})

	t.Run("aggregates by project", func(t *testing.T) {
		score1 := float32(0.95)
		score2 := float32(0.80)
		chunks := []Chunk{
			{ProjectName: "Alpha", Section: "problem", Score: &score1},
			{ProjectName: "Alpha", Section: "outcome", Score: &score2},
			{ProjectName: "Beta", Section: "decisions"},
		}
		matches := aggregateMatches(chunks)
		if len(matches) != 2 {
			t.Fatalf("expected 2 matches, got %d", len(matches))
		}
		// Alpha should be first (higher relevance)
		if matches[0].ProjectName != "Alpha" {
			t.Fatalf("expected Alpha first, got %s", matches[0].ProjectName)
		}
		if len(matches[0].Sections) != 2 {
			t.Fatalf("expected 2 sections for Alpha, got %d", len(matches[0].Sections))
		}
	})

	t.Run("sorts by relevance descending", func(t *testing.T) {
		chunks := []Chunk{
			{ProjectName: "C", Section: "outcome", Score: float32Ptr(0.50)},
			{ProjectName: "A", Section: "outcome", Score: float32Ptr(0.95)},
			{ProjectName: "B", Section: "outcome", Score: float32Ptr(0.80)},
		}
		matches := aggregateMatches(chunks)
		if len(matches) != 3 {
			t.Fatalf("expected 3 matches, got %d", len(matches))
		}
		if matches[0].Relevance != 0.95 || matches[1].Relevance != 0.80 || matches[2].Relevance != 0.50 {
			t.Fatalf("expected sorted by relevance, got %+v", matches)
		}
	})
}

func TestGeneratePitch(t *testing.T) {
	t.Run("returns empty when LLM is nil", func(t *testing.T) {
		exec := NewToolExecutor(&mockVectorStore{}, &mockEmbedder{}, nil, "projects")
		pitch, err := exec.generatePitch(context.Background(), "jd", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if pitch != "" {
			t.Fatalf("expected empty pitch, got %q", pitch)
		}
	})

	t.Run("returns empty when no matches", func(t *testing.T) {
		exec := NewToolExecutor(&mockVectorStore{}, &mockEmbedder{}, &mockChatClient{}, "projects")
		pitch, err := exec.generatePitch(context.Background(), "jd", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if pitch != "" {
			t.Fatalf("expected empty pitch, got %q", pitch)
		}
	})
}

func TestMatchJobDescriptionRequiresJDText(t *testing.T) {
	exec := NewToolExecutor(&mockVectorStore{}, &mockEmbedder{}, nil, "projects")
	_, err := exec.Execute(context.Background(), toolMatchJobDescription, `{}`)
	if err == nil {
		t.Fatal("expected error for missing jd_text")
	}
}

func TestMatchJobDescriptionEmptyJDText(t *testing.T) {
	exec := NewToolExecutor(&mockVectorStore{}, &mockEmbedder{}, nil, "projects")
	_, err := exec.Execute(context.Background(), toolMatchJobDescription, `{"jd_text":"  "}`)
	if err == nil {
		t.Fatal("expected error for empty jd_text")
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

func TestMatchJobDescriptionFull(t *testing.T) {
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

func TestAgentMaxIter(t *testing.T) {
	t.Run("defaults to 5", func(t *testing.T) {
		agent := &Agent{}
		if agent.maxIter() != 5 {
			t.Fatalf("expected 5, got %d", agent.maxIter())
		}
	})

	t.Run("uses custom value", func(t *testing.T) {
		agent := &Agent{MaxIter: 10}
		if agent.maxIter() != 10 {
			t.Fatalf("expected 10, got %d", agent.maxIter())
		}
	})
}

func TestAgentBaseMessages(t *testing.T) {
	agent := &Agent{SystemPrompt: "test prompt"}
	msgs := agent.baseMessages([]Message{{Role: "user", Content: "hello"}})
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Role != "system" {
		t.Fatalf("expected system message first, got %s", msgs[0].Role)
	}
	if msgs[0].Content != "test prompt" {
		t.Fatalf("expected system prompt, got %q", msgs[0].Content)
	}
}

func TestAgentRunExceedsMaxIter(t *testing.T) {
	agent := &Agent{
		MaxIter: 1,
		LLM: &mockChatClient{
			responses: []Message{
				{
					Role: "assistant",
					ToolCalls: []ToolCall{
						{ID: "call_1", Type: "function", Function: FunctionCall{Name: "test", Arguments: "{}"}},
					},
				},
			},
		},
		Tools: ToolExecutor{},
	}
	_, err := agent.Run(context.Background(), []Message{{Role: "user", Content: "hello"}})
	if err == nil {
		t.Fatal("expected error for exceeding max iterations")
	}
	if !strings.Contains(err.Error(), "exceeded max iterations") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAgentRunStreamExceedsMaxIter(t *testing.T) {
	agent := &Agent{
		MaxIter: 1,
		LLM: &mockChatClient{
			responses: []Message{
				{
					Role: "assistant",
					ToolCalls: []ToolCall{
						{ID: "call_1", Type: "function", Function: FunctionCall{Name: "test", Arguments: "{}"}},
					},
				},
			},
		},
		Tools: ToolExecutor{},
	}
	_, err := agent.RunStream(context.Background(), []Message{{Role: "user", Content: "hello"}}, nil)
	if err == nil {
		t.Fatal("expected error for exceeding max iterations")
	}
	if !strings.Contains(err.Error(), "exceeded max iterations") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAgentRunStreamWithOnToken(t *testing.T) {
	store := &mockVectorStore{
		queryFn: func(_ context.Context, _ string, _ []float32, _ *Filter, _ uint64) ([]Chunk, error) {
			return []Chunk{{ProjectName: "Alpha", Section: "outcome", ChunkText: "Delivered"}}, nil
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
			{Role: "assistant", Content: "done"},
		},
		streamTokens: []string{"streamed ", "answer"},
	}

	agent := NewAgent(llm, *NewToolExecutor(store, &mockEmbedder{}, nil, "projects"), "system")
	var tokens []string
	result, err := agent.RunStream(context.Background(), []Message{{Role: "user", Content: "Tell me"}}, func(token string) error {
		tokens = append(tokens, token)
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Join(tokens, "") != "streamed answer" {
		t.Fatalf("unexpected tokens: %v", tokens)
	}
	if result.Content != "streamed answer" {
		t.Fatalf("unexpected content: %q", result.Content)
	}
}
