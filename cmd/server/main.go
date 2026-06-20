package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ragsume-core/agentkit"
	"ragsume-core/config"
	"ragsume-core/logger"
)

func main() {
	if err := config.Load(); err != nil {
		_ = logger.Init(logger.Options{Level: "error", Format: logger.FormatText})
		logger.Fatal("failed to load config", "error", err)
	}

	if err := logger.Init(logger.Options{
		AppName: config.C.AppName,
		Level:   config.C.LogLevel,
		Format:  logger.Format(config.C.LogFormat),
		Debug:   config.C.Debug,
		LogFile: config.C.LogFile,
	}); err != nil {
		_ = logger.Init(logger.Options{Level: "error", Format: logger.FormatText})
		logger.Fatal("failed to initialize logger", "error", err)
	}
	defer func() { _ = logger.Close() }()

	profile, err := LoadProfile("data/profile.yaml")
	if err != nil {
		logger.Fatal("load profile", "error", err)
	}
	profileYAML, err := RenderProfileYAML(profile)
	if err != nil {
		logger.Fatal("render profile", "error", err)
	}

	qdrantClient, err := agentkit.NewQdrantClient(config.C.QdrantURL, config.C.QdrantAPIKey)
	if err != nil {
		logger.Fatal("connect qdrant", "error", err)
	}
	defer func() { _ = qdrantClient.Close() }()
	// Log successful connection to Qdrant
	logger.Info("connected to qdrant", "url", config.C.QdrantURL)

	embedder := agentkit.NewOllamaEmbedder(config.C.OllamaURL, config.DefaultEmbedModel)
	llm := agentkit.NewOpenRouterClient(config.C.OpenRouterAPIKey, config.DefaultLLMModel)
	tools := agentkit.NewToolExecutor(qdrantClient, embedder, llm, config.DefaultCollectionName)
	agent := agentkit.NewAgent(llm, *tools, BuildSystemPrompt(profileYAML))

	router := newRouter(agent)
	addr := fmt.Sprintf(":%d", config.C.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 0,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.Info("server listening", "addr", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("server error", "error", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	logger.Info("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("shutdown error", "error", err)
	}
}
