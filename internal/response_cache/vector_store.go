package responsecache

import (
	"context"
	"fmt"
	"strings"
	"time"

	"aurora/configuration"
)

type VecResult struct {
	Key        string
	Score      float32
	Response   []byte
	PromptText string
}

type VecStore interface {
	Search(ctx context.Context, vec []float32, paramsHash string, limit int) ([]VecResult, error)
	Insert(ctx context.Context, key string, vec []float32, response []byte, paramsHash string, promptText string, ttl time.Duration) error
	DeleteExpired(ctx context.Context) error
	Close() error
}

func NewVecStore(cfg config.VectorStoreConfig) (VecStore, error) {
	t := strings.TrimSpace(cfg.Type)
	if t == "" {
		return nil, fmt.Errorf("vecstore: vector_store.type is required (qdrant, pgvector, pinecone, weaviate)")
	}
	switch t {
	case "qdrant":
		return newQdrantStore(cfg.Qdrant)
	case "pgvector":
		return newPGVectorStore(cfg.PGVector)
	case "pinecone":
		return newPineconeStore(cfg.Pinecone)
	case "weaviate":
		return newWeaviateStore(cfg.Weaviate)
	default:
		return nil, fmt.Errorf("vecstore: unknown backend type %q (valid: qdrant, pgvector, pinecone, weaviate)", t)
	}
}
