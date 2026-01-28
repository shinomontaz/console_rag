package chunker

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Factory создаёт chunker на основе метода и типа файла
type Factory struct {
	config Config
}

// NewFactory создаёт новую фабрику chunker'ов
func NewFactory(config Config) *Factory {
	return &Factory{config: config}
}

// GetChunker возвращает подходящий chunker для файла
func (f *Factory) GetChunker(filePath, method string) (Chunker, error) {
	// Если метод явно указан - используем его
	switch strings.ToLower(method) {
	case "markdown", "md":
		return NewMarkdownChunker(f.config), nil
	case "simple", "text", "txt":
		return NewTextChunker(f.config), nil
	}

	// Иначе определяем по расширению файла
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".md", ".markdown":
		return NewMarkdownChunker(f.config), nil
	case ".txt", ".text":
		return NewTextChunker(f.config), nil
	default:
		return NewTextChunker(f.config), nil
	}
}

// GetChunkerByMethod возвращает chunker по названию метода
func (f *Factory) GetChunkerByMethod(method string) (Chunker, error) {
	switch strings.ToLower(method) {
	case "markdown", "md":
		return NewMarkdownChunker(f.config), nil
	case "simple", "text", "txt":
		return NewTextChunker(f.config), nil
	default:
		return nil, fmt.Errorf("unknown chunking method: %s", method)
	}
}
