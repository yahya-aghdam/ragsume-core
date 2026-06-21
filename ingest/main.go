package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ragsume-core/agentkit"
	"ragsume-core/config"
	"ragsume-core/logger"
)

func main() {
	dataDir := flag.String("data-dir", "data/projects", "directory containing project YAML files")
	collection := flag.String("collection", config.DefaultCollectionName, "Qdrant collection name")
	flag.Parse()

	if err := config.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	if err := logger.Init(logger.Options{
		AppName: config.C.AppName,
		Level:   config.C.LogLevel,
		Format:  logger.Format(config.C.LogFormat),
		Debug:   config.C.Debug,
		LogFile: config.C.LogFile,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "init logger: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = logger.Close() }()

	ctx := context.Background()

	qdrantClient, err := agentkit.NewQdrantClient(config.C.QdrantURL, config.C.QdrantAPIKey)
	if err != nil {
		logger.Fatal("connect qdrant", "error", err)
	}
	defer func() { _ = qdrantClient.Close() }()

	embedder := agentkit.NewOllamaEmbedder(config.C.OllamaURL, config.DefaultEmbedModel)

	if err := qdrantClient.EnsureCollection(ctx, *collection, config.DefaultVectorSize); err != nil {
		logger.Fatal("ensure collection", "error", err)
	}
	if err := qdrantClient.EnsurePayloadIndexes(ctx, *collection, []string{"tech_stack", "category", "section", "project_name"}); err != nil {
		logger.Fatal("ensure payload indexes", "error", err)
	}

	files, err := filepath.Glob(filepath.Join(*dataDir, "*.yaml"))
	if err != nil {
		logger.Fatal("glob project files", "error", err)
	}
	if len(files) == 0 {
		logger.Fatal("no project yaml files found", "data_dir", *dataDir)
	}

	totalPoints := 0
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			logger.Fatal("read project file", "file", file, "error", err)
		}

		project, err := parseProject(data)
		if err != nil {
			logger.Fatal("parse project", "file", file, "error", err)
		}

		chunks := chunkProject(project)
		if len(chunks) == 0 {
			logger.Warn("skipping project with no embeddable sections", "project", project.ProjectName, "file", file)
			continue
		}

		techStack := normalizeTechStack(project.TechStack)
		category := strings.ToLower(strings.TrimSpace(project.Category))
		points := make([]agentkit.PointInput, 0, len(chunks))

		for _, chunk := range chunks {
			vector, err := embedder.Embed(ctx, chunk.Text)
			if err != nil {
				logger.Fatal("embed chunk", "project", project.ProjectName, "section", chunk.Section, "error", err)
			}

			points = append(points, agentkit.PointInput{
				ID:     pointID(project.ProjectName, chunk.Section),
				Vector: vector,
				Payload: map[string]any{
					"project_name": project.ProjectName,
					"category":     category,
					"tech_stack":   techStack,
					"date_range":   project.DateRange,
					"chunk_text":   chunk.Text,
					"section":      chunk.Section,
					"tradeoffs":    project.Tradeoffs,
				},
			})
		}

		if err := qdrantClient.Upsert(ctx, *collection, points); err != nil {
			logger.Fatal("upsert project", "project", project.ProjectName, "error", err)
		}

		totalPoints += len(points)
		logger.Info("ingested project", "project", project.ProjectName, "chunks", len(points), "file", file)
	}

	logger.Info("ingest complete", "projects", len(files), "points", totalPoints, "collection", *collection)
}
