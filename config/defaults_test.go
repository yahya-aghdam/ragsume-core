package config

import (
	"testing"
)

func TestDefaultConstants(t *testing.T) {
	t.Run("DefaultCollectionName", func(t *testing.T) {
		if DefaultCollectionName != "projects" {
			t.Fatalf("got %q, want projects", DefaultCollectionName)
		}
	})

	t.Run("DefaultEmbedModel", func(t *testing.T) {
		if DefaultEmbedModel != "nomic-embed-text" {
			t.Fatalf("got %q, want nomic-embed-text", DefaultEmbedModel)
		}
	})

	t.Run("DefaultVectorSize", func(t *testing.T) {
		if DefaultVectorSize != 768 {
			t.Fatalf("got %d, want 768", DefaultVectorSize)
		}
	})

	t.Run("DefaultLLMModel", func(t *testing.T) {
		if DefaultLLMModel != "openai/gpt-oss-120b:free" {
			t.Fatalf("got %q, want openai/gpt-oss-120b:free", DefaultLLMModel)
		}
	})
}
