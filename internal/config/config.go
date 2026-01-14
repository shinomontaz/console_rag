package config

import (
	"github.com/caarlos0/env/v10"
)

type Config struct {
	ReferenceDoc     string `env:"REFERENCE_DOC"`
	DataDir          string `env:"DATA_DIR" envDefault:"./data"`
	OllamaURL        string `env:"OLLAMA_URL" envDefault:"http://localhost:11434"`
	OllamaModel      string `env:"OLLAMA_MODEL" envDefault:"gemma2:2b"`
	OllamaEmbedModel string `env:"OLLAMA_EMBED_MODEL" envDefault:"nomic-embed-text"`
	ChunkSize        int    `env:"CHUNK_SIZE" envDefault:"1000"`
	ChunkOverlap     int    `env:"CHUNK_OVERLAP" envDefault:"200"`
	MetadataFile     string
	DBFile           string
}

func Init(cfg interface{}) error {
	return env.Parse(cfg)
}
