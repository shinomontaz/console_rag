package config

import (
	"strings"

	"github.com/caarlos0/env/v10"
)

type Llm struct {
	Type  string `env:"TYPE" envDefault:"openai"` // openai или gemini
	URL   string `env:"URL"`                      // для openai (llama.cpp/qwen)
	Model string `env:"MODEL,required"`
	Key   string `env:"KEY"`
}

type Promt struct {
	Header string `env:"HEADER"`
	Footer string `env:"FOOTER"`
	Etalon string `env:"ETALON"`
	Chunk  string `env:"CHUNK"`
}

type Config struct {
	ReferenceDoc string `env:"REFERENCE_DOC"`
	DataDir      string `env:"DATA_DIR" envDefault:"./data"`
	LlmMain      Llm    `envPrefix:"LLM_MAIN_"`
	LlmEmbed     Llm    `envPrefix:"LLM_EMBED_"`
	ChunkMethod  string `env:"CHUNK_METHOD" envDefault:"markdown"`
	ChunkSize    int    `env:"CHUNK_SIZE" envDefault:"1000"`
	ChunkOverlap int    `env:"CHUNK_OVERLAP" envDefault:"200"`
	CustomPromt  Promt  `envPrefix:"CUSTOM_PROMPT_"`

	// Параметры векторного поиска
	TopK          int     `env:"TOP_K" envDefault:"5"`
	MinSimilarity float32 `env:"MIN_SIMILARITY" envDefault:"0.6"`

	// Параметры LLM (оптимизировано для gemma3)
	MaxTokens      int     `env:"MAX_TOKENS" envDefault:"2000"`
	Temperature    float32 `env:"TEMPERATURE" envDefault:"0.3"`
	MaxPromptChars int     `env:"MAX_PROMPT_CHARS" envDefault:"10000"`

	// Параметры параллельной обработки
	MaxConcurrency int `env:"MAX_CONCURRENCY" envDefault:"3"`
}

func Init(cfg *Config) error {
	err := env.Parse(cfg)
	if err != nil {
		return err
	}

	// Убираем кавычки если есть
	cfg.CustomPromt.Header = strings.ReplaceAll(strings.Trim(cfg.CustomPromt.Header, `"`), "\\n", "\n")
	cfg.CustomPromt.Chunk = strings.ReplaceAll(strings.Trim(cfg.CustomPromt.Chunk, `"`), "\\n", "\n")
	cfg.CustomPromt.Etalon = strings.ReplaceAll(strings.Trim(cfg.CustomPromt.Etalon, `"`), "\\n", "\n")
	cfg.CustomPromt.Footer = strings.ReplaceAll(strings.Trim(cfg.CustomPromt.Footer, `"`), "\\n", "\n")

	// Применяем дефолты для CustomPromt если пусто
	if cfg.CustomPromt.Header == "" {
		cfg.CustomPromt.Header = "Сравни проверяемый текст с эталоном. Найди несоответствия."
	}
	if cfg.CustomPromt.Chunk == "" {
		cfg.CustomPromt.Chunk = "ПРОВЕРЯЕМЫЙ ТЕКСТ:"
	}
	if cfg.CustomPromt.Etalon == "" {
		cfg.CustomPromt.Etalon = "ЭТАЛОН:"
	}
	if cfg.CustomPromt.Footer == "" {
		cfg.CustomPromt.Footer = "Что не совпадает?\nОтвет:\nСтатус: ✅/⚠️/❌\nНесоответствия: ...\nИсправления: ..."
	}

	return nil
}
