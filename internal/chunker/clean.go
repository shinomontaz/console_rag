package chunker

import (
	"log"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Кэшированный regexp для замены множественных переносов
var reMultipleNewlines = regexp.MustCompile(`\n{3,}`)

func cleanChunk(content string) string {
	// Убрать лишние пробелы и переносы
	content = strings.TrimSpace(content)

	// Убрать обрезанные слова в начале (если начинается с маленькой буквы)
	if len(content) > 0 {
		r, _ := utf8.DecodeRuneInString(content)
		if unicode.IsLower(r) {
			removed := strings.Fields(content)[0]
			parts := strings.Fields(content)
			if len(parts) > 1 {
				content = strings.Join(parts[1:], " ")
				log.Printf("🔧 cleanChunk: removed leading word %q (starts with lowercase)", removed)
			}
		}
	}

	// Убрать обрезанные слова в конце
	content = strings.TrimRight(content, " -")

	// Заменить множественные переносы на двойной
	content = reMultipleNewlines.ReplaceAllString(content, "\n\n")

	// Убрать лишние пробелы между словами (НЕ трогаем переносы!)
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lines[i] = strings.Join(strings.Fields(line), " ")
	}
	content = strings.Join(lines, "\n")

	return content
}
