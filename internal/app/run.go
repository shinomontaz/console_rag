package app

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (a *App) Run(ctx context.Context) error {
	log.Println("Application started")
	log.Println("Enter text to analyze (one per line). Ctrl+C to exit.")

	scanner := bufio.NewScanner(os.Stdin)

	// Увеличим буфер, если пути/строки будут длинные
	const maxLineSize = 1024 * 1024
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, maxLineSize)

	for {
		select {
		case <-ctx.Done():
			log.Println("Shutting down application")
			return nil
		default:
			// читаем строку
			if !scanner.Scan() {
				if err := scanner.Err(); err != nil {
					return fmt.Errorf("stdin error: %w", err)
				}
				// EOF
				log.Println("stdin closed")
				return nil
			}

			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}

			// Каждая строка = путь к файлу
			a.handleFile(line)
		}
	}
}

func (a *App) handleFile(path string) {
	log.Printf("Received input: %s", path)

	ctx := context.Background()

	// Проверяем, это файл или текст
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		// Это файл - обрабатываем через processInputDocument
		ext := filepath.Ext(path)
		if ext != ".md" && ext != ".txt" && ext != ".pdf" {
			log.Printf("❌ Unsupported format: %s", ext)
			return
		}

		// Определяем путь для сохранения результатов
		if a.outputPath == "" {
			// Автоматическое имя файла
			timestamp := time.Now().Format("20060102_150405")
			baseName := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
			a.outputPath = fmt.Sprintf("%s_analysis_%s.md", baseName, timestamp)
		}

		if err := a.processInputDocument(ctx, path); err != nil {
			log.Printf("❌ Processing failed: %v", err)
		}

		// Сбрасываем outputPath для следующего файла
		a.outputPath = ""
		return
	}

	// Это просто текст - обрабатываем как раньше
	results, err := a.searchRelevantChunks(ctx, path)
	if err != nil {
		log.Printf("❌ Search error: %v", err)
		return
	}

	log.Printf("🔍 Found %d relevant sections:", len(results))
	for i, r := range results {
		log.Printf("   %d. %s (similarity: %.2f)", i+1, r.Section, r.Similarity)
	}

	log.Printf("\n🤖 Analyzing with LLM...")
	prompt := a.buildAnalysisPrompt(path, results)

	analysis, err := a.queryLLM(ctx, prompt)
	if err != nil {
		log.Printf("❌ LLM error: %v", err)
		return
	}

	log.Printf("\n%s\n", analysis)
}
