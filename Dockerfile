# syntax=docker/dockerfile:1

FROM golang:1.25-bookworm AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/server ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/ingest ./ingest

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates wget \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /out/server /app/server
COPY --from=builder /out/ingest /app/ingest
COPY data/profile.yaml /app/data/profile.yaml
COPY data/projects /app/data/projects

ENV PORT=8080
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD ["/bin/sh", "-c", "wget -qO- http://127.0.0.1:${PORT}/health || exit 1"]

CMD ["/app/server"]
