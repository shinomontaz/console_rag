package chunker

import (
	"regexp"
	"strings"
	"unicode"
)

func cleanChunk(content string) string {
	// Убрать лишние пробелы и переносы
	content = strings.TrimSpace(content)

	// Убрать обрезанные слова в начале (если начинается с маленькой буквы)
	if len(content) > 0 && unicode.IsLower(rune(content[0])) {
		parts := strings.Fields(content)
		if len(parts) > 1 {
			content = strings.Join(parts[1:], " ")
		}
	}

	// Убрать обрезанные слова в конце
	content = strings.TrimRight(content, " -")

	// Заменить множественные переносы на двойной
	re := regexp.MustCompile(`\n{3,}`)
	content = re.ReplaceAllString(content, "\n\n")

	// Убрать лишние пробелы между словами (НЕ трогаем переносы!)
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lines[i] = strings.Join(strings.Fields(line), " ")
	}
	content = strings.Join(lines, "\n")

	return content
}
