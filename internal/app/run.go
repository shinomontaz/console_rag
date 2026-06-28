package app

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (a *App) Run(ctx context.Context) error {
	a.logger.Infof("Application started")

	if a.cfg.CheckDoc != "" {
		a.logger.Infof("Processing check document: %s", a.cfg.CheckDoc)
		if _, err := os.Stat(a.cfg.CheckDoc); os.IsNotExist(err) {
			return fmt.Errorf("check document not found: %s", a.cfg.CheckDoc)
		}

		ext := filepath.Ext(a.cfg.CheckDoc)
		if ext != ".md" && ext != ".txt" && ext != ".pdf" {
			return fmt.Errorf("unsupported format: %s", ext)
		}

		if a.outputPath == "" {
			timestamp := time.Now().Format("20060102_150405")
			baseName := strings.TrimSuffix(filepath.Base(a.cfg.CheckDoc), filepath.Ext(a.cfg.CheckDoc))
			a.outputPath = fmt.Sprintf("%s_analysis_%s.md", baseName, timestamp)
		}

		if err := a.processInputDocument(ctx, a.cfg.CheckDoc); err != nil {
			return fmt.Errorf("failed to process check document: %w", err)
		}

		a.logger.Infof("Finished processing check document\n")
		a.logger.Infof("Shutting down application")

		return nil
	}

	a.logger.Infof("Enter text to analyze (one per line). Ctrl+C to exit.")

	scanner := bufio.NewScanner(os.Stdin)

	const maxLineSize = 1024 * 1024
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, maxLineSize)

	for {
		select {
		case <-ctx.Done():
			a.logger.Infof("Shutting down application")

			return nil
		default:
			if !scanner.Scan() {
				if err := scanner.Err(); err != nil {
					return fmt.Errorf("stdin error: %w", err)
				}
				a.logger.Infof("stdin closed")

				return nil
			}

			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}

			// Каждая строка = путь к файлу
			a.handleFile(ctx, line)
		}
	}
}

func (a *App) handleFile(ctx context.Context, path string) {
	a.logger.Infof("Received input: %s", path)

	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		ext := filepath.Ext(path)
		if ext != ".md" && ext != ".txt" && ext != ".pdf" {
			a.logger.Errorf("❌ Unsupported format: %s", ext)
			return
		}

		if a.outputPath == "" {
			timestamp := time.Now().Format("20060102_150405")
			baseName := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
			a.outputPath = fmt.Sprintf("%s_analysis_%s.md", baseName, timestamp)
		}

		if err := a.processInputDocument(ctx, path); err != nil {
			a.logger.Errorf("❌ Processing failed: %v", err)
		}

		a.outputPath = ""

		return
	}

	// Это просто текст - обрабатываем как раньше
	results, err := a.searchRelevantChunks(ctx, path)
	if err != nil {
		a.logger.Errorf("❌ Search error: %v", err)
		return
	}

	a.logger.Infof("🔍 Found %d relevant sections:", len(results))
	for i, r := range results {
		a.logger.Infof("   %d. %s (similarity: %.2f)", i+1, r.Section, r.Similarity)
	}

	a.logger.Infof("\n🤖 Analyzing with LLM...")
	prompt := a.buildAnalysisPrompt(path, results)

	analysis, err := a.queryLLM(ctx, prompt)
	if err != nil {
		a.logger.Errorf("❌ LLM error: %v", err)
		return
	}

	a.logger.Infof("\n%s", analysis)
}
