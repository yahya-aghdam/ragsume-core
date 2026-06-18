package agentkit

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const (
	toolSearchProfile       = "search_profile"
	toolMatchJobDescription = "match_job_description"
	defaultCollection       = "projects"
	defaultQueryLimit       = 10
	defaultMatchLimit       = 8
)

var defaultTools = []ToolDefinition{
	{
		Type: "function",
		Function: FunctionDefinition{
			Name: toolSearchProfile,
			Description: "Search project history chunks. Populate filter when the user names a " +
				"specific technology, language, or category. Use an empty query with a filter " +
				"for metadata-only listing.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "Semantic search query. Leave empty for metadata-only scroll when filter is set.",
					},
					"filter": map[string]any{
						"type":        "object",
						"description": "Optional Qdrant-style filter with must conditions.",
						"properties": map[string]any{
							"must": map[string]any{
								"type": "array",
								"items": map[string]any{
									"type": "object",
									"properties": map[string]any{
										"field": map[string]any{"type": "string"},
										"match": map[string]any{"type": "string"},
									},
									"required": []string{"field", "match"},
								},
							},
						},
					},
				},
			},
		},
	},
	{
		Type: "function",
		Function: FunctionDefinition{
			Name:        toolMatchJobDescription,
			Description: "Match a job description to the most relevant project experience and produce a short tailored pitch.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"jd_text": map[string]any{
						"type":        "string",
						"description": "Full job description text.",
					},
				},
				"required": []string{"jd_text"},
			},
		},
	},
}

type searchProfileArgs struct {
	Query  string  `json:"query"`
	Filter *Filter `json:"filter"`
}

type searchProfileResult struct {
	Chunks []Chunk `json:"chunks"`
}

type jobMatch struct {
	ProjectName string   `json:"project_name"`
	Sections    []string `json:"sections"`
	Relevance   float32  `json:"relevance"`
}

type matchJobDescriptionResult struct {
	Matches []jobMatch `json:"matches"`
	Pitch   string     `json:"pitch"`
}

// ToolExecutor executes agent tools against a vector store.
type ToolExecutor struct {
	Store      VectorStore
	Embedder   Embedder
	LLM        ChatClient
	Collection string
}

func NewToolExecutor(store VectorStore, embedder Embedder, llm ChatClient, collection string) *ToolExecutor {
	if collection == "" {
		collection = defaultCollection
	}
	return &ToolExecutor{
		Store:      store,
		Embedder:   embedder,
		LLM:        llm,
		Collection: collection,
	}
}

func (e *ToolExecutor) Execute(ctx context.Context, name, arguments string) (string, error) {
	switch name {
	case toolSearchProfile:
		return e.searchProfile(ctx, arguments)
	case toolMatchJobDescription:
		return e.matchJobDescription(ctx, arguments)
	default:
		return "", fmt.Errorf("unknown tool %q", name)
	}
}

func (e *ToolExecutor) searchProfile(ctx context.Context, arguments string) (string, error) {
	var args searchProfileArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return "", fmt.Errorf("parse search_profile args: %w", err)
	}

	query := strings.TrimSpace(args.Query)
	hasFilter := args.Filter != nil && len(args.Filter.Must) > 0
	if query == "" && !hasFilter {
		return "", fmt.Errorf("search_profile requires a query, a filter, or both")
	}

	var chunks []Chunk
	var err error

	switch {
	case query == "" && hasFilter:
		chunks, err = e.Store.Scroll(ctx, e.Collection, args.Filter, defaultQueryLimit)
	case query != "":
		vector, embedErr := e.Embedder.Embed(ctx, query)
		if embedErr != nil {
			return "", fmt.Errorf("embed search query: %w", embedErr)
		}
		chunks, err = e.Store.Query(ctx, e.Collection, vector, args.Filter, defaultQueryLimit)
	}

	if err != nil {
		return "", err
	}

	out, err := json.Marshal(searchProfileResult{Chunks: chunks})
	if err != nil {
		return "", fmt.Errorf("marshal search_profile result: %w", err)
	}
	return string(out), nil
}

func (e *ToolExecutor) matchJobDescription(ctx context.Context, arguments string) (string, error) {
	var args struct {
		JDText string `json:"jd_text"`
	}
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return "", fmt.Errorf("parse match_job_description args: %w", err)
	}
	jdText := strings.TrimSpace(args.JDText)
	if jdText == "" {
		return "", fmt.Errorf("jd_text is required")
	}

	vector, err := e.Embedder.Embed(ctx, jdText)
	if err != nil {
		return "", fmt.Errorf("embed job description: %w", err)
	}

	chunks, err := e.Store.Query(ctx, e.Collection, vector, nil, defaultMatchLimit)
	if err != nil {
		return "", err
	}

	matches := aggregateMatches(chunks)
	pitch, err := e.generatePitch(ctx, jdText, matches)
	if err != nil {
		return "", err
	}

	out, err := json.Marshal(matchJobDescriptionResult{
		Matches: matches,
		Pitch:   pitch,
	})
	if err != nil {
		return "", fmt.Errorf("marshal match_job_description result: %w", err)
	}
	return string(out), nil
}

func aggregateMatches(chunks []Chunk) []jobMatch {
	type agg struct {
		sections  map[string]struct{}
		relevance float32
	}
	byProject := map[string]*agg{}

	for _, chunk := range chunks {
		entry, ok := byProject[chunk.ProjectName]
		if !ok {
			entry = &agg{sections: map[string]struct{}{}}
			byProject[chunk.ProjectName] = entry
		}
		entry.sections[chunk.Section] = struct{}{}
		if chunk.Score != nil && *chunk.Score > entry.relevance {
			entry.relevance = *chunk.Score
		}
	}

	matches := make([]jobMatch, 0, len(byProject))
	for name, entry := range byProject {
		sections := make([]string, 0, len(entry.sections))
		for section := range entry.sections {
			sections = append(sections, section)
		}
		sort.Strings(sections)
		matches = append(matches, jobMatch{
			ProjectName: name,
			Sections:    sections,
			Relevance:   entry.relevance,
		})
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Relevance > matches[j].Relevance
	})
	return matches
}

func (e *ToolExecutor) generatePitch(ctx context.Context, jdText string, matches []jobMatch) (string, error) {
	if e.LLM == nil || len(matches) == 0 {
		return "", nil
	}

	names := make([]string, 0, len(matches))
	for _, m := range matches {
		names = append(names, m.ProjectName)
	}

	msg, err := e.LLM.Complete(ctx, ChatCompletionRequest{
		Messages: []Message{
			{
				Role: "system",
				Content: "Write a concise 2-3 sentence recruiter pitch grounded only in the " +
					"provided project names. Do not invent experience.",
			},
			{
				Role:    "user",
				Content: fmt.Sprintf("Job description:\n%s\n\nRelevant projects: %s", jdText, strings.Join(names, ", ")),
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("generate pitch: %w", err)
	}
	return strings.TrimSpace(msg.Content), nil
}

func DefaultTools() []ToolDefinition {
	return append([]ToolDefinition(nil), defaultTools...)
}
