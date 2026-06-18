package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"ragsume-core/agentkit"
	"ragsume-core/cmd/server/handlers"
	appmiddleware "ragsume-core/cmd/server/middleware"
)

func newRouter(agent *agentkit.Agent) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(appmiddleware.CORS)
	// Log each incoming HTTP request
	r.Use(appmiddleware.RequestLogger)

	r.Get("/health", handlers.Health)
	r.Post("/chat", handlers.NewChatHandler(agent).ServeHTTP)

	return r
}
