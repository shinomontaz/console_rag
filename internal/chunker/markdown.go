package chunker

import (
	"fmt"
	"log"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// MarkdownChunker разбивает markdown документы с адаптивным выбором стратегии
type MarkdownChunker struct {
	config Config
}

// NewMarkdownChunker создаёт новый markdown chunker
func NewMarkdownChunker(config Config) *MarkdownChunker {
	return &MarkdownChunker{config: config}
}

func (m *MarkdownChunker) Name() string {
	return "markdown"
}

// DocumentStructure содержит информацию о структуре документа
type DocumentStructure struct {
	HeadingCounts   map[int]int // уровень заголовка -> количество
	TotalParagraphs int
	TotalSize       int
}

// ChunkingStrategy определяет стратегию разбиения
type ChunkingStrategy struct {
	Level int // уровень заголовка (2-4)
}

func (m *MarkdownChunker) Chunk(content, source string) ([]Chunk, error) {
	md := goldmark.New()
	reader := text.NewReader([]byte(content))
	doc := md.Parser().Parse(reader)

	// Анализируем структуру документа
	structure := m.analyzeStructure(doc)

	// Выбираем стратегию разбиения
	strategy, err := m.selectStrategy(structure)
	if err != nil {
		// Явно возвращаем ошибку - пусть вызывающий код решает что делать
		return nil, fmt.Errorf("markdown chunker cannot process this content: %w", err)
	}

	log.Printf("📊 [%s] Document structure: headings=%v, paragraphs=%d",
		m.Name(), structure.HeadingCounts, structure.TotalParagraphs)

	var chunks []Chunk
	if strategy.Level == 0 {
		// Разбиваем по параграфам
		log.Printf("🎯 [%s] Selected strategy: paragraphs (AST)", m.Name())

		chunks = m.chunkByParagraphsAST(doc, []byte(content), source)
	} else {
		// Применяем стратегию разбиения по заголовкам
		log.Printf("🎯 [%s] Selected strategy: heading (level %d)", m.Name(), strategy.Level)
		chunks = m.chunkByHeadings(doc, []byte(content), source, strategy.Level)
	}

	log.Printf("✅ [%s] Created %d chunks", m.Name(), len(chunks))
	return chunks, nil
}

// analyzeStructure анализирует структуру markdown документа
func (m *MarkdownChunker) analyzeStructure(doc ast.Node) DocumentStructure {
	structure := DocumentStructure{
		HeadingCounts: make(map[int]int),
	}

	ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			if heading, ok := n.(*ast.Heading); ok {
				structure.HeadingCounts[heading.Level]++
			}
			if _, ok := n.(*ast.Paragraph); ok {
				structure.TotalParagraphs++
			}
		}
		return ast.WalkContinue, nil
	})

	return structure
}

// selectStrategy выбирает уровень заголовков для разбиения
func (m *MarkdownChunker) selectStrategy(structure DocumentStructure) (ChunkingStrategy, error) {
	// Проверяем заголовки от H2 до H4 (наиболее частые для структурированных документов)
	for level := 2; level <= 4; level++ {
		count := structure.HeadingCounts[level]
		// Если есть достаточно заголовков этого уровня - используем их
		minHeadings := 3
		switch level {
		case 2:
			minHeadings = 3 // Для H2 (статьи) достаточно 3
		case 3:
			minHeadings = 5 // Для H3 (подразделы) нужно больше
		default:
			minHeadings = 10 // Для H4 нужно ещё больше
		}

		if count >= minHeadings {
			return ChunkingStrategy{Level: level}, nil
		}
	}

	if structure.TotalParagraphs >= 2 {
		return ChunkingStrategy{Level: 0}, nil // Level=0 означает "по параграфам"
	}

	// Нет подходящей markdown структуры - возвращаем ошибку
	return ChunkingStrategy{}, fmt.Errorf(
		"no suitable markdown structure found (headings: %v, paragraphs: %d)",
		structure.HeadingCounts, structure.TotalParagraphs,
	)
}

func (m *MarkdownChunker) chunkByParagraphsAST(doc ast.Node, content []byte, source string) []Chunk {
	var paragraphs []string

	// Собираем параграфы из AST — каждый параграф гарантированно целый
	ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			if p, ok := n.(*ast.Paragraph); ok {
				text := string(p.Text(content))
				if strings.TrimSpace(text) != "" {
					paragraphs = append(paragraphs, text)
				}
			}
		}
		return ast.WalkContinue, nil
	})

	// Группируем параграфы в чанки до MaxChunkSize
	var chunks []Chunk
	var currentPart strings.Builder
	chunkNum := 1
	var prevParagraphs []string // для overlap — целые параграфы!

	for _, para := range paragraphs {
		if currentPart.Len() > 0 && currentPart.Len()+len(para) > m.config.MaxChunkSize {
			// Сохраняем чанк
			chunks = append(chunks, CreateChunk(currentPart.String(), source,
				fmt.Sprintf("Чанк %d", chunkNum), map[string]string{
					"method": "paragraphs-ast",
				}))

			currentPart.Reset()

			// Overlap: берём последние 1-2 целых параграфа, а не N символов
			overlapParas := m.selectOverlapParagraphs(prevParagraphs)
			for _, op := range overlapParas {
				currentPart.WriteString(op)
				currentPart.WriteString("\n\n")
			}

			chunkNum++
		}

		prevParagraphs = append(prevParagraphs, para)
		if len(prevParagraphs) > 5 { // держим только последние 5 для overlap
			prevParagraphs = prevParagraphs[1:]
		}

		if currentPart.Len() > 0 {
			currentPart.WriteString("\n\n")
		}
		currentPart.WriteString(para)
	}

	// Последний чанк
	if currentPart.Len() > 0 {
		chunks = append(chunks, CreateChunk(currentPart.String(), source,
			fmt.Sprintf("Чанк %d", chunkNum), map[string]string{
				"method": "paragraphs-ast",
			}))
	}

	return chunks
}

// продвинутое перекрытие по целым параграфам
func (m *MarkdownChunker) selectOverlapParagraphs(recentParas []string) []string {
	if m.config.Overlap <= 0 || len(recentParas) == 0 {
		return nil
	}

	// Берём параграфы с конца, пока не превысим Overlap по размеру
	var result []string
	totalLen := 0
	for i := len(recentParas) - 1; i >= 0; i-- {
		if totalLen+len(recentParas[i]) > m.config.Overlap {
			break
		}
		result = append([]string{recentParas[i]}, result...)
		totalLen += len(recentParas[i])
	}
	return result
}

// chunkByHeadings разбивает документ по заголовкам указанного уровня
func (m *MarkdownChunker) chunkByHeadings(doc ast.Node, content []byte, source string, targetLevel int) []Chunk {
	var chunks []Chunk
	var currentChunk strings.Builder
	var currentSection string
	var parentSection string // Для контекста подразделов
	var currentLevel int

	ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			if heading, ok := n.(*ast.Heading); ok {
				headingText := extractText(heading, content)

				// Если встретили заголовок целевого уровня или выше - начинаем новый чанк
				if heading.Level <= targetLevel {
					if currentChunk.Len() > 0 {
						// Сохраняем предыдущий чанк
						chunks = append(chunks, m.finalizeChunk(
							currentChunk.String(),
							source,
							currentSection,
							parentSection,
							currentLevel,
						)...)
						currentChunk.Reset()
					}

					currentSection = headingText
					currentLevel = heading.Level

					// Обновляем родительскую секцию
					if heading.Level == targetLevel {
						parentSection = headingText
					}

					currentChunk.WriteString(headingText + "\n\n")
				} else {
					// Подзаголовки включаем в текущий чанк
					currentChunk.WriteString("\n" + headingText + "\n\n")
				}
			} else if textNode, ok := n.(*ast.Text); ok {
				// Пропускаем Text-узлы внутри Heading — они уже записаны как headingText
				if _, isHeading := textNode.Parent().(*ast.Heading); !isHeading {
					currentChunk.Write(textNode.Segment.Value(content))
				}
			}
		} else {
			if _, ok := n.(*ast.Paragraph); ok {
				currentChunk.WriteString("\n\n")
			}
		}
		return ast.WalkContinue, nil
	})

	// Сохраняем последний чанк
	if currentChunk.Len() > 0 {
		chunks = append(chunks, m.finalizeChunk(
			currentChunk.String(),
			source,
			currentSection,
			parentSection,
			currentLevel,
		)...)
	}

	return chunks
}

// finalizeChunk обрабатывает чанк: разбивает если большой, добавляет overlap для подразделов
func (m *MarkdownChunker) finalizeChunk(text, source, section, parentSection string, level int) []Chunk {
	text = strings.TrimSpace(text)

	// Если чанк меньше лимита - возвращаем как есть
	if len(text) <= m.config.MaxChunkSize {
		metadata := map[string]string{
			"level": fmt.Sprintf("%d", level),
		}
		if parentSection != "" && parentSection != section {
			metadata["parent_section"] = parentSection
		}
		return []Chunk{CreateChunk(text, source, section, metadata)}
	}

	// Разбиваем большой чанк на части по параграфам
	return m.splitLargeChunk(text, source, section, parentSection, level)
}

func (m *MarkdownChunker) splitLargeChunk(text, source, section, parentSection string, level int) []Chunk {
	paragraphs := SplitByParagraphs(text)
	var chunks []Chunk
	var currentPart strings.Builder
	partNum := 1
	var prevParagraphs []string // ← было: var prevTail string

	useOverlap := level > 2 && m.config.Overlap > 0

	for _, para := range paragraphs {
		if currentPart.Len() > 0 && currentPart.Len()+len(para) > m.config.MaxChunkSize {
			partText := currentPart.String()

			sectionWithPart := section
			if partNum > 1 {
				sectionWithPart = fmt.Sprintf("%s (часть %d)", section, partNum)
			}

			metadata := map[string]string{
				"level":     fmt.Sprintf("%d", level),
				"part":      fmt.Sprintf("%d", partNum),
				"has_parts": "true",
			}
			if parentSection != "" && parentSection != section {
				metadata["parent_section"] = parentSection
			}

			chunks = append(chunks, CreateChunk(partText, source, sectionWithPart, metadata))

			currentPart.Reset()

			if useOverlap {
				overlapParas := m.selectOverlapParagraphs(prevParagraphs)
				for _, op := range overlapParas {
					currentPart.WriteString(op)
					currentPart.WriteString("\n\n")
				}
			}

			partNum++
		}

		// Накапливаем параграфы для overlap
		prevParagraphs = append(prevParagraphs, para)
		if len(prevParagraphs) > 5 {
			prevParagraphs = prevParagraphs[1:]
		}

		if currentPart.Len() > 0 {
			currentPart.WriteString("\n\n")
		}
		currentPart.WriteString(para)
	}

	// Последний чанк — без изменений
	if currentPart.Len() > 0 {
		sectionWithPart := section
		if partNum > 1 {
			sectionWithPart = fmt.Sprintf("%s (часть %d)", section, partNum)
		}

		metadata := map[string]string{
			"level": fmt.Sprintf("%d", level),
		}
		if partNum > 1 {
			metadata["part"] = fmt.Sprintf("%d", partNum)
			metadata["has_parts"] = "true"
		}
		if parentSection != "" && parentSection != section {
			metadata["parent_section"] = parentSection
		}

		chunks = append(chunks, CreateChunk(currentPart.String(), source, sectionWithPart, metadata))
	}

	return chunks
}

// extractText извлекает текст из узла AST
func extractText(node ast.Node, source []byte) string {
	var buf strings.Builder
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		if textNode, ok := child.(*ast.Text); ok {
			buf.Write(textNode.Segment.Value(source))
		}
	}
	return buf.String()
}
