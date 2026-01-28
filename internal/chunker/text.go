package chunker

import (
	"fmt"
	"log"
	"strings"
)

// TextChunker разбивает plain text по размеру с overlap
type TextChunker struct {
	config Config
}

// NewTextChunker создаёт новый simple chunker
func NewTextChunker(config Config) *TextChunker {
	return &TextChunker{config: config}
}

func (s *TextChunker) Name() string {
	return "simple"
}

func (s *TextChunker) Chunk(content, source string) ([]Chunk, error) {
	// Пытаемся разбить по параграфам если они есть
	if strings.Contains(content, "\n\n") {
		chunks := s.chunkByParagraphs(content, source)
		log.Printf("✅ [%s] Created %d chunks (by paragraphs)", s.Name(), len(chunks))
		return chunks, nil
	}

	// Иначе простое разбиение по размеру
	chunks := s.chunkBySize(content, source)
	log.Printf("✅ [%s] Created %d chunks (by size)", s.Name(), len(chunks))
	return chunks, nil
}

// chunkByParagraphs разбивает текст по параграфам с overlap
func (s *TextChunker) chunkByParagraphs(content, source string) []Chunk {
	paragraphs := SplitByParagraphs(content)
	var chunks []Chunk
	var currentChunk strings.Builder
	chunkNum := 1
	var prevTail string

	for _, para := range paragraphs {
		// Если добавление параграфа превысит лимит
		if currentChunk.Len() > 0 && currentChunk.Len()+len(para) > s.config.MaxChunkSize {
			chunkText := currentChunk.String()
			section := fmt.Sprintf("Чанк %d", chunkNum)

			chunks = append(chunks, CreateChunk(chunkText, source, section, map[string]string{
				"chunk_num": fmt.Sprintf("%d", chunkNum),
				"method":    "paragraphs",
			}))

			// Сохраняем хвост для overlap
			if s.config.Overlap > 0 {
				prevTail = GetLastNChars(chunkText, s.config.Overlap)
			}

			currentChunk.Reset()

			// Добавляем overlap
			if prevTail != "" {
				currentChunk.WriteString(prevTail)
				currentChunk.WriteString("\n\n")
			}

			chunkNum++
		}

		if currentChunk.Len() > 0 {
			currentChunk.WriteString("\n\n")
		}
		currentChunk.WriteString(para)
	}

	// Последний чанк
	if currentChunk.Len() > 0 {
		section := fmt.Sprintf("Чанк %d", chunkNum)
		chunks = append(chunks, CreateChunk(currentChunk.String(), source, section, map[string]string{
			"chunk_num": fmt.Sprintf("%d", chunkNum),
			"method":    "paragraphs",
		}))
	}

	return chunks
}

// chunkBySize простое разбиение по размеру с overlap
func (s *TextChunker) chunkBySize(content, source string) []Chunk {
	var chunks []Chunk
	runes := []rune(content)
	chunkNum := 1

	for i := 0; i < len(runes); i += s.config.MaxChunkSize - s.config.Overlap {
		end := i + s.config.MaxChunkSize
		if end > len(runes) {
			end = len(runes)
		}

		chunkText := string(runes[i:end])
		section := fmt.Sprintf("Чанк %d", chunkNum)

		chunks = append(chunks, CreateChunk(chunkText, source, section, map[string]string{
			"chunk_num": fmt.Sprintf("%d", chunkNum),
			"method":    "size",
		}))

		chunkNum++

		if end >= len(runes) {
			break
		}
	}

	return chunks
}
