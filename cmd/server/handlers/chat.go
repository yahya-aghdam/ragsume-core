package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"ragsume-core/agentkit"
)

type chatRequest struct {
	Message string           `json:"message"`
	History []agentkit.Message `json:"history"`
}

type tokenEvent struct {
	Content string `json:"content"`
}

// ChatHandler serves POST /chat with SSE streaming.
type ChatHandler struct {
	Agent *agentkit.Agent
}

func NewChatHandler(agent *agentkit.Agent) *ChatHandler {
	return &ChatHandler{Agent: agent}
}

func (h *ChatHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}
	if req.Message == "" {
		http.Error(w, "message is required", http.StatusBadRequest)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	messages := append([]agentkit.Message(nil), req.History...)
	messages = append(messages, agentkit.Message{Role: "user", Content: req.Message})

	_, err := h.Agent.RunStream(r.Context(), messages, func(token string) error {
		payload, err := json.Marshal(tokenEvent{Content: token})
		if err != nil {
			return fmt.Errorf("marshal token event: %w", err)
		}
		if _, err := fmt.Fprintf(w, "event: token\ndata: %s\n\n", payload); err != nil {
			return fmt.Errorf("write token event: %w", err)
		}
		flusher.Flush()
		return nil
	})
	if err != nil {
		errPayload, _ := json.Marshal(map[string]string{"error": err.Error()})
		_, _ = fmt.Fprintf(w, "event: error\ndata: %s\n\n", errPayload)
		flusher.Flush()
		return
	}

	_, _ = fmt.Fprint(w, "event: done\ndata: {}\n\n")
	flusher.Flush()
}
