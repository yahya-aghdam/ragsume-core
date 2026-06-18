package agentkit

import (
	"context"
	"fmt"

	qdrant "github.com/qdrant/go-client/qdrant"
)

// VectorStore abstracts Qdrant operations used by tools and ingest.
type VectorStore interface {
	EnsureCollection(ctx context.Context, name string, vectorSize uint64) error
	EnsurePayloadIndexes(ctx context.Context, collection string, fields []string) error
	Upsert(ctx context.Context, collection string, points []PointInput) error
	Scroll(ctx context.Context, collection string, filter *Filter, limit uint64) ([]Chunk, error)
	Query(ctx context.Context, collection string, vector []float32, filter *Filter, limit uint64) ([]Chunk, error)
	Close() error
}

// QdrantClient wraps the official Qdrant Go client.
type QdrantClient struct {
	client *qdrant.Client
}

// NewQdrantClient connects to Qdrant using a URL and optional API key.
func NewQdrantClient(rawURL, apiKey string) (*QdrantClient, error) {
	host, port, useTLS, err := ParseQdrantURL(rawURL)
	if err != nil {
		return nil, err
	}

	client, err := qdrant.NewClient(&qdrant.Config{
		Host:     host,
		Port:     port,
		APIKey:   apiKey,
		UseTLS:   useTLS,
		PoolSize: 1,
	})
	if err != nil {
		return nil, fmt.Errorf("create qdrant client: %w", err)
	}

	return &QdrantClient{client: client}, nil
}

func (c *QdrantClient) Close() error {
	if c.client == nil {
		return nil
	}
	return c.client.Close()
}

func (c *QdrantClient) EnsureCollection(ctx context.Context, name string, vectorSize uint64) error {
	exists, err := c.client.CollectionExists(ctx, name)
	if err != nil {
		return fmt.Errorf("check collection exists: %w", err)
	}
	if exists {
		return nil
	}

	err = c.client.CreateCollection(ctx, &qdrant.CreateCollection{
		CollectionName: name,
		VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
			Size:     vectorSize,
			Distance: qdrant.Distance_Cosine,
		}),
	})
	if err != nil {
		return fmt.Errorf("create collection: %w", err)
	}
	return nil
}

func (c *QdrantClient) EnsurePayloadIndexes(ctx context.Context, collection string, fields []string) error {
	wait := true
	for _, field := range fields {
		_, err := c.client.CreateFieldIndex(ctx, &qdrant.CreateFieldIndexCollection{
			CollectionName: collection,
			FieldName:      field,
			FieldType:      qdrant.FieldType_FieldTypeKeyword.Enum(),
			Wait:           &wait,
		})
		if err != nil {
			return fmt.Errorf("create payload index on %q: %w", field, err)
		}
	}
	return nil
}

func (c *QdrantClient) Upsert(ctx context.Context, collection string, points []PointInput) error {
	if len(points) == 0 {
		return nil
	}

	qdrantPoints := make([]*qdrant.PointStruct, 0, len(points))
	for _, p := range points {
		payload, err := qdrant.TryValueMap(normalizePayload(p.Payload))
		if err != nil {
			return fmt.Errorf("convert payload for point %q: %w", p.ID, err)
		}
		qdrantPoints = append(qdrantPoints, &qdrant.PointStruct{
			Id:      qdrant.NewIDUUID(p.ID),
			Vectors: qdrant.NewVectorsDense(p.Vector),
			Payload: payload,
		})
	}

	wait := true
	_, err := c.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: collection,
		Points:         qdrantPoints,
		Wait:           &wait,
	})
	if err != nil {
		return fmt.Errorf("upsert points: %w", err)
	}
	return nil
}

func (c *QdrantClient) Scroll(ctx context.Context, collection string, filter *Filter, limit uint64) ([]Chunk, error) {
	if limit == 0 {
		limit = 20
	}
	limit32 := uint32(limit)

	req := &qdrant.ScrollPoints{
		CollectionName: collection,
		Limit:          qdrant.PtrOf(limit32),
		WithPayload:    qdrant.NewWithPayload(true),
	}
	if qf := filter.ToQdrant(); qf != nil {
		req.Filter = qf
	}

	points, err := c.client.Scroll(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("scroll points: %w", err)
	}

	return chunksFromRetrieved(points), nil
}

func (c *QdrantClient) Query(ctx context.Context, collection string, vector []float32, filter *Filter, limit uint64) ([]Chunk, error) {
	if limit == 0 {
		limit = 10
	}
	limit32 := uint32(limit)

	req := &qdrant.QueryPoints{
		CollectionName: collection,
		Query:          qdrant.NewQueryDense(vector),
		Limit:          qdrant.PtrOf(uint64(limit32)),
		WithPayload:    qdrant.NewWithPayload(true),
	}
	if qf := filter.ToQdrant(); qf != nil {
		req.Filter = qf
	}

	points, err := c.client.Query(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("query points: %w", err)
	}

	return chunksFromScored(points), nil
}

func chunksFromRetrieved(points []*qdrant.RetrievedPoint) []Chunk {
	out := make([]Chunk, 0, len(points))
	for _, p := range points {
		out = append(out, chunkFromPayload(p.GetPayload(), nil))
	}
	return out
}

func chunksFromScored(points []*qdrant.ScoredPoint) []Chunk {
	out := make([]Chunk, 0, len(points))
	for _, p := range points {
		score := p.GetScore()
		out = append(out, chunkFromPayload(p.GetPayload(), &score))
	}
	return out
}
