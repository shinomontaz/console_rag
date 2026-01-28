package chunker

import (
	"crypto/sha256"
	"fmt"
	"strings"
)

// CreateChunk создаёт чанк с автоматической генерацией ID
func CreateChunk(text, source, section string, metadata map[string]string) Chunk {
	text = strings.TrimSpace(text)
	hash := sha256.Sum256([]byte(text + source))

	if metadata == nil {
		metadata = make(map[string]string)
	}

	return Chunk{
		ID:       fmt.Sprintf("%x", hash[:8]),
		Text:     text,
		Source:   source,
		Section:  section,
		Metadata: metadata,
	}
}

// GetLastNChars возвращает последние N символов строки для overlap
func GetLastNChars(text string, n int) string {
	runes := []rune(text)
	if len(runes) <= n {
		return text
	}
	return string(runes[len(runes)-n:])
}

// SplitByParagraphs разбивает текст на параграфы
func SplitByParagraphs(text string) []string {
	paragraphs := strings.Split(text, "\n\n")
	var result []string
	for _, p := range paragraphs {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
