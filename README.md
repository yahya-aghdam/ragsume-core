# Ragsume Core

## Overview

Ragsume Core is a Go‑based backend service that provides a flexible, AI‑driven chat
interface. It includes a configurable server, a set of tools for interacting with
vector stores, embedding models, and a simple ingestion pipeline for loading project
metadata.

## Technology Stack

| Component | Technology |
|-----------|------------|
| Language  | Go (1.22+) |
| HTTP Server | net/http with chi router |
| Embeddings | OpenAI embeddings API (or compatible provider) |
| Vector Store | Qdrant (via REST API) |
| Configuration | Viper for env/YAML handling |
| Logging | Zap structured logger |
| Testing | Go's testing package, testify |
| Containerisation | Docker |

## Capabilities

* **Context‑aware chat** – Maintains conversation history and uses retrieved
  relevant documents from the vector store to augment responses.
* **RAG (Retrieval‑Augmented Generation)** – Stores project documentation in a
  vector database and performs similarity search to provide up‑to‑date answers.
* **Multi‑project ingestion** – Parses YAML project definitions (see `data/projects`)
  and indexes their content for retrieval.
* **Extensible tool system** – Agent can call additional tools (e.g., Qdrant
  search, embedding generation) defined in `agentkit/tools.go`.
* **Docker ready** – Can be built and run as a container for easy deployment.
* **Configurable** – All settings (port, API keys, vector store URL) are driven by
  environment variables or a `.env` file.

The project is organized into several packages:

* **`cmd/server`** – The HTTP server exposing the chat API.
* **`agentkit`** – Core agent logic, embedding helpers, and tool integrations.
* **`config`** – Configuration loading from environment variables and YAML files.
* **`logger`** – Structured logging utilities.
* **`ingest`** – Utilities for ingesting project data into a vector store.

## Prerequisites

* Go 1.22 or later
* Docker (optional, for containerised deployment)
* An OpenAI API key (or compatible provider) – set via `OPENAI_API_KEY`
* Qdrant instance (or other vector store) – configure in `config/defaults.go`

## Getting Started

### 1. Clone the repository

```bash
git clone https://github.com/yourorg/ragsume-core.git
cd ragsume-core
```

### 2. Install dependencies

```bash
go mod tidy
```

### 3. Set up environment variables

Copy the example file and fill in the required values:

```bash
cp .env.example .env
# Edit .env and set OPENAI_API_KEY, QDRANT_URL, etc.
```

### 4. Run the server locally

```bash
go run ./cmd/server
```

The server will start on `http://localhost:8080`. You can test the health endpoint:

```bash
curl http://localhost:8080/health
```

### 5. Ingest project data (optional)

If you have a `data/projects/*.yaml` file describing a project, you can ingest it
into the vector store:

```bash
go run ./ingest -project data/projects/ragsume.yaml
```

## Configuration

Configuration is loaded in the following order (first wins):

1. Environment variables (`APP_` prefix)
2. `.env` file in the project root
3. `config/defaults.go` – sensible defaults for development

Key settings include:

| Variable | Description |
|----------|-------------|
| `APP_PORT` | Port for the HTTP server (default `8080`) |
| `OPENAI_API_KEY` | API key for the embedding model |
| `QDRANT_URL` | URL of the Qdrant vector store |
| `QDRANT_API_KEY` | Optional API key for Qdrant |

## API Endpoints

### Health Check

`GET /health`

Returns `200 OK` with a JSON payload `{ "status": "ok" }`.

### Chat

`POST /chat`

Request body:

```json
{
  "messages": [{"role": "user", "content": "Your question"}]
}
```

Response body:

```json
{
  "reply": "Generated answer..."
}
```

The endpoint forwards the conversation to the agent defined in `agentkit/agent.go`
which may call external tools (e.g., Qdrant search) to produce a response.

## Development

### Running Tests

```bash
go test ./...
```

### Linting & Formatting

```bash
go fmt ./...
golangci-lint run
```

### Docker

A `Dockerfile` is provided for containerised deployment:

```bash
docker build -t ragsume-core .
docker run -p 8080:8080 --env-file .env ragsume-core
```

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/foo`)
3. Commit your changes (`git commit -am 'Add foo'`)
4. Push to the branch (`git push origin feature/foo`)
5. Open a Pull Request

Please ensure that new code includes unit tests and passes `go vet`.

## License

This project is licensed under the MIT License – see the `LICENSE` file for details.

---
