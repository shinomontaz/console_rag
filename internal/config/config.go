package config

import (
	"github.com/caarlos0/env/v10"
)

type Llm struct {
	URL   string `env:"URL,required"`
	Model string `env:"MODEL,required"`
	Key   string `env:"KEY"`
}

type Config struct {
	ReferenceDoc string `env:"REFERENCE_DOC"`
	DataDir      string `env:"DATA_DIR" envDefault:"./data"`
	LlmMain      Llm    `envPrefix:"LLM_MAIN_"`
	LlmEmbed     Llm    `envPrefix:"LLM_EMBED_"`
	ChunkMethod  string `env:"CHUNK_METHOD" envDefault:"markdown"`
	ChunkSize    int    `env:"CHUNK_SIZE" envDefault:"1000"`
	ChunkOverlap int    `env:"CHUNK_OVERLAP" envDefault:"200"`
	
	// Параметры векторного поиска
	TopK          int     `env:"TOP_K" envDefault:"5"`
	MinSimilarity float32 `env:"MIN_SIMILARITY" envDefault:"0.6"`
	
	// Параметры LLM (оптимизировано для gemma3)
	MaxTokens      int     `env:"MAX_TOKENS" envDefault:"2000"`
	Temperature    float32 `env:"TEMPERATURE" envDefault:"0.3"`
	MaxPromptChars int     `env:"MAX_PROMPT_CHARS" envDefault:"24000"`
	
	// Параметры параллельной обработки
	MaxConcurrency int `env:"MAX_CONCURRENCY" envDefault:"3"`
}

func Init(cfg interface{}) error {
	return env.Parse(cfg)
}
