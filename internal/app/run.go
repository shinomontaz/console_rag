package app

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"path/filepath"
)

func (a *App) Run(ctx context.Context) error {
	log.Println("Application started")
	log.Println("Enter file paths (one per line). Ctrl+C to exit.")

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

	info, err := os.Stat(path)
	if err != nil {
		log.Printf("File error: %v", err)
		return
	}

	if info.IsDir() {
		log.Printf("Skipping directory: %s", path)
		return
	}

	if filepath.Ext(path) == ".md" {
		log.Printf("File exthension is .md, no need to convert")
	}

	if filepath.Ext(path) == ".pdf" {
		log.Printf("File exthension is .pdf, converting PDF to text, it can take a while")
	}

	if filepath.Ext(path) == ".docx" {
		log.Printf("This is M$ Word file, convertion would take a while...")
	}

	// Пока просто логируем
	log.Printf("Would process file: %s (%d bytes)", path, info.Size())

	// В будущем:
	// - прогнать через LegalChunker
	// - сделать retrieval
	// - отправить в LLM
}
