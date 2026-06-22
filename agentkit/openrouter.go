package agentkit

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const openRouterBaseURL = "https://openrouter.ai/api/v1/chat/completions"

// Message is an OpenAI-compatible chat message.
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`
}

// ToolCall represents a model-requested tool invocation.
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

// FunctionCall holds the tool name and JSON arguments.
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ToolDefinition describes a callable tool for the model.
type ToolDefinition struct {
	Type     string             `json:"type"`
	Function FunctionDefinition `json:"function"`
}

// FunctionDefinition is the schema exposed to the model.
type FunctionDefinition struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  any    `json:"parameters"`
}

// ChatCompletionRequest is sent to OpenRouter.
type ChatCompletionRequest struct {
	Model    string           `json:"model"`
	Messages []Message        `json:"messages"`
	Tools    []ToolDefinition `json:"tools,omitempty"`
	Stream   bool             `json:"stream,omitempty"`
}

// ChatCompletionResponse is a non-streaming completion result.
type ChatCompletionResponse struct {
	Choices []struct {
		Message      Message `json:"message"`
		FinishReason string  `json:"finish_reason"`
	} `json:"choices"`
}

// ChatClient performs chat completions against an OpenAI-compatible API.
type ChatClient interface {
	Complete(ctx context.Context, req ChatCompletionRequest) (Message, error)
	CompleteStream(ctx context.Context, req ChatCompletionRequest, onToken func(string) error) (Message, error)
}

// OpenRouterClient calls OpenRouter's chat completions endpoint.
type OpenRouterClient struct {
	APIKey     string
	Model      string
	HTTPClient *http.Client
	Referer    string
}

// NewOpenRouterClient creates a client with defaults.
func NewOpenRouterClient(apiKey, model string) *OpenRouterClient {
	if model == "" {
		model = "google/gemini-2.5-flash"
	}
	return &OpenRouterClient{
		APIKey:  apiKey,
		Model:   model,
		Referer: "https://ragsume-core",
		HTTPClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

func (c *OpenRouterClient) Complete(ctx context.Context, req ChatCompletionRequest) (Message, error) {
	req.Model = c.model(req.Model)
	req.Stream = false

	body, err := json.Marshal(req)
	if err != nil {
		return Message{}, fmt.Errorf("marshal chat request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, openRouterBaseURL, bytes.NewReader(body))
	if err != nil {
		return Message{}, fmt.Errorf("create chat request: %w", err)
	}
	c.setHeaders(httpReq)

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return Message{}, fmt.Errorf("chat request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return Message{}, fmt.Errorf("chat returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	var parsed ChatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return Message{}, fmt.Errorf("decode chat response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return Message{}, fmt.Errorf("chat returned no choices")
	}

	return parsed.Choices[0].Message, nil
}

func (c *OpenRouterClient) CompleteStream(ctx context.Context, req ChatCompletionRequest, onToken func(string) error) (Message, error) {
	req.Model = c.model(req.Model)
	req.Stream = true

	body, err := json.Marshal(req)
	if err != nil {
		return Message{}, fmt.Errorf("marshal chat request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, openRouterBaseURL, bytes.NewReader(body))
	if err != nil {
		return Message{}, fmt.Errorf("create chat request: %w", err)
	}
	c.setHeaders(httpReq)

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return Message{}, fmt.Errorf("chat stream request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return Message{}, fmt.Errorf("chat stream returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	var content strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return Message{}, fmt.Errorf("decode stream chunk: %w", err)
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		token := chunk.Choices[0].Delta.Content
		if token == "" {
			continue
		}
		content.WriteString(token)
		if onToken != nil {
			if err := onToken(token); err != nil {
				return Message{}, err
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return Message{}, fmt.Errorf("read chat stream: %w", err)
	}

	return Message{Role: "assistant", Content: content.String()}, nil
}

func (c *OpenRouterClient) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	if c.Referer != "" {
		req.Header.Set("HTTP-Referer", c.Referer)
	}
}

func (c *OpenRouterClient) model(requestModel string) string {
	if requestModel != "" {
		return requestModel
	}
	return c.Model
}
