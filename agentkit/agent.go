package agentkit

import (
	"context"
	"fmt"
)

const defaultMaxIterations = 5

// Agent runs a tool-calling loop against an OpenAI-compatible chat API.
type Agent struct {
	LLM          ChatClient
	Tools        ToolExecutor
	SystemPrompt string
	MaxIter      int
	ToolDefs     []ToolDefinition
}

// RunResult is the final assistant response from a non-streaming run.
type RunResult struct {
	Content  string
	Messages []Message
}

// NewAgent creates an agent with defaults.
func NewAgent(llm ChatClient, tools ToolExecutor, systemPrompt string) *Agent {
	return &Agent{
		LLM:          llm,
		Tools:        tools,
		SystemPrompt: systemPrompt,
		MaxIter:      defaultMaxIterations,
		ToolDefs:     DefaultTools(),
	}
}

func (a *Agent) baseMessages(userMessages []Message) []Message {
	msgs := make([]Message, 0, len(userMessages)+1)
	msgs = append(msgs, Message{Role: "system", Content: a.SystemPrompt})
	msgs = append(msgs, userMessages...)
	return msgs
}

// Run executes the tool loop and returns the final assistant message.
func (a *Agent) Run(ctx context.Context, userMessages []Message) (*RunResult, error) {
	messages := a.baseMessages(userMessages)

	for i := 0; i < a.maxIter(); i++ {
		resp, err := a.LLM.Complete(ctx, ChatCompletionRequest{
			Messages: messages,
			Tools:    a.ToolDefs,
		})
		if err != nil {
			return nil, fmt.Errorf("chat completion: %w", err)
		}

		if len(resp.ToolCalls) == 0 {
			messages = append(messages, resp)
			return &RunResult{
				Content:  resp.Content,
				Messages: messages,
			}, nil
		}

		messages = append(messages, resp)
		for _, call := range resp.ToolCalls {
			result, err := a.Tools.Execute(ctx, call.Function.Name, call.Function.Arguments)
			if err != nil {
				result = fmt.Sprintf("error: %v", err)
			}
			messages = append(messages, Message{
				Role:       "tool",
				ToolCallID: call.ID,
				Name:       call.Function.Name,
				Content:    result,
			})
		}
	}

	return nil, fmt.Errorf("agent exceeded max iterations (%d)", a.maxIter())
}

// RunStream executes tool rounds without streaming, then streams the final answer.
func (a *Agent) RunStream(ctx context.Context, userMessages []Message, onToken func(string) error) (*RunResult, error) {
	messages := a.baseMessages(userMessages)
	readyToStream := false

	for i := 0; i < a.maxIter(); i++ {
		resp, err := a.LLM.Complete(ctx, ChatCompletionRequest{
			Messages: messages,
			Tools:    a.ToolDefs,
		})
		if err != nil {
			return nil, fmt.Errorf("chat completion: %w", err)
		}

		if len(resp.ToolCalls) == 0 {
			readyToStream = true
			break
		}

		messages = append(messages, resp)
		for _, call := range resp.ToolCalls {
			result, err := a.Tools.Execute(ctx, call.Function.Name, call.Function.Arguments)
			if err != nil {
				result = fmt.Sprintf("error: %v", err)
			}
			messages = append(messages, Message{
				Role:       "tool",
				ToolCallID: call.ID,
				Name:       call.Function.Name,
				Content:    result,
			})
		}
	}

	if !readyToStream {
		return nil, fmt.Errorf("agent exceeded max iterations (%d)", a.maxIter())
	}

	streamed, err := a.LLM.CompleteStream(ctx, ChatCompletionRequest{
		Messages: messages,
	}, onToken)
	if err != nil {
		return nil, fmt.Errorf("chat stream: %w", err)
	}
	messages = append(messages, streamed)
	return &RunResult{
		Content:  streamed.Content,
		Messages: messages,
	}, nil
}

func (a *Agent) maxIter() int {
	if a.MaxIter <= 0 {
		return defaultMaxIterations
	}
	return a.MaxIter
}
