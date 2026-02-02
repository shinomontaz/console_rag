package app

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"console_rag/internal/chunker"
)

// processInputDocument обрабатывает входной файл
func (a *App) processInputDocument(ctx context.Context, filePath string) error {
	// Читаем файл
	content, err := readFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	log.Printf("📄 File loaded: %d bytes", len(content))

	// Определяем chunker
	chunkr, err := a.chunkerFactory.GetChunker(filePath, a.cfg.ChunkMethod)
	if err != nil {
		return fmt.Errorf("failed to get chunker: %w", err)
	}

	// Разбиваем на чанки
	chunks, err := chunkr.Chunk(content, filepath.Base(filePath))
	if err != nil {
		// Fallback на text chunker
		log.Printf("⚠️  Chunker failed: %v, falling back to text chunker", err)
		textChunker := chunker.NewTextChunker(chunker.Config{
			MaxChunkSize: a.cfg.ChunkSize,
			Overlap:      a.cfg.ChunkOverlap,
		})
		chunks, err = textChunker.Chunk(content, filepath.Base(filePath))
		if err != nil {
			return fmt.Errorf("text chunker failed: %w", err)
		}
	}

	log.Printf("📦 Split into %d chunks\n", len(chunks))

	// Semaphore для контроля concurrency
	sem := make(chan struct{}, a.cfg.MaxConcurrency)

	// Mutex для упорядоченного вывода
	var mu sync.Mutex
	results := make([]*AnalysisResult, len(chunks))

	// WaitGroup для ожидания завершения
	var wg sync.WaitGroup

	for i, chunk := range chunks {
		wg.Add(1)
		go func(idx int, ch chunker.Chunk) {
			defer wg.Done()

			// Захватываем semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			result := &AnalysisResult{
				ChunkIndex:   idx + 1,
				ChunkSection: ch.Section,
			}

			// Поиск релевантных секций
			searchResults, err := a.searchRelevantChunks(ctx, ch.Text)
			if err != nil {
				result.Error = err
				mu.Lock()
				results[idx] = result
				mu.Unlock()
				return
			}

			result.ReferenceCount = len(searchResults)

			// Анализ через LLM
			prompt := a.buildAnalysisPrompt(ch.Text, searchResults)
			analysis, err := a.queryLLM(ctx, prompt)
			if err != nil {
				result.Error = err
				mu.Lock()
				results[idx] = result
				mu.Unlock()
				return
			}

			result.Analysis = analysis

			// Сохраняем результат
			mu.Lock()
			results[idx] = result
			mu.Unlock()

			// Выводим результат (thread-safe)
			mu.Lock()
			log.Printf("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
			log.Printf("Chunk %d/%d: %s", result.ChunkIndex, len(chunks), result.ChunkSection)
			log.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
			log.Printf("🔍 Found %d relevant sections", result.ReferenceCount)
			log.Printf("\n🤖 Analysis:\n%s\n", result.Analysis)
			mu.Unlock()
		}(i, chunk)
	}

	wg.Wait()

	// Итоговая статистика
	successCount := 0
	errorCount := 0
	for _, r := range results {
		if r != nil {
			if r.Error != nil {
				errorCount++
			} else {
				successCount++
			}
		}
	}

	log.Printf("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Printf("📊 Summary:")
	log.Printf("   Total chunks: %d", len(chunks))
	log.Printf("   ✅ Analyzed: %d", successCount)
	log.Printf("   ❌ Errors: %d", errorCount)
	log.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

	// Сохраняем результаты в файл
	if a.outputPath != "" {
		analysis := &DocumentAnalysis{
			FileName:     filepath.Base(filePath),
			TotalChunks:  len(chunks),
			Results:      results,
			SuccessCount: successCount,
			ErrorCount:   errorCount,
			ProcessedAt:  time.Now().Format("2006-01-02 15:04:05"),
		}

		if err := saveAnalysisResults(analysis, a.outputPath); err != nil {
			log.Printf("⚠️  Failed to save results: %v", err)
		} else {
			log.Printf("💾 Results saved to: %s", a.outputPath)
		}
	}

	return nil
}

// AnalysisResult - результат анализа чанка
type AnalysisResult struct {
	ChunkIndex     int
	ChunkSection   string
	Analysis       string
	ReferenceCount int
	Error          error
}

// DocumentAnalysis - полный результат анализа документа
type DocumentAnalysis struct {
	FileName     string
	TotalChunks  int
	Results      []*AnalysisResult
	SuccessCount int
	ErrorCount   int
	ProcessedAt  string
}

// saveAnalysisResults сохраняет результаты в файл
func saveAnalysisResults(analysis *DocumentAnalysis, outputPath string) error {
	var buf strings.Builder

	buf.WriteString(fmt.Sprintf("# Анализ документа: %s\n\n", analysis.FileName))
	buf.WriteString(fmt.Sprintf("**Дата анализа:** %s\n\n", analysis.ProcessedAt))
	buf.WriteString(fmt.Sprintf("**Всего чанков:** %d\n\n", analysis.TotalChunks))

	// Итоговая статистика
	buf.WriteString("## Итоговая статистика\n\n")
	buf.WriteString(fmt.Sprintf("- ✅ Проанализировано: %d\n", analysis.SuccessCount))
	buf.WriteString(fmt.Sprintf("- ❌ Ошибок: %d\n\n", analysis.ErrorCount))

	// Детальные результаты
	buf.WriteString("## Детальный анализ\n\n")
	for _, result := range analysis.Results {
		if result == nil || result.Error != nil {
			continue
		}

		buf.WriteString(fmt.Sprintf("### Chunk %d: %s\n\n", result.ChunkIndex, result.ChunkSection))
		buf.WriteString(fmt.Sprintf("**Релевантных секций найдено:** %d\n\n", result.ReferenceCount))
		buf.WriteString("**Анализ:**\n\n")
		buf.WriteString(result.Analysis)
		buf.WriteString("\n\n---\n\n")
	}

	return os.WriteFile(outputPath, []byte(buf.String()), 0644)
}
