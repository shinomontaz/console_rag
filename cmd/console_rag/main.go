package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"console_rag/internal/app"
	"console_rag/internal/config"

	"github.com/joho/godotenv"
)

func main() {
	// Парсим флаги командной строки
	referenceDoc := flag.String("reference-doc", "", "Path to reference document (required)")
	dataDir := flag.String("data", "./data", "Data directory for vector DB")
	outputFile := flag.String("output", "", "Save analysis results to file (optional)")
	flag.Parse()

	if *referenceDoc == "" {
		log.Fatal("Error: --reference-doc flag is required\nUsage: console_rag --reference-doc=/path/to/document.md")
	}

	// Проверяем существование файла
	if _, err := os.Stat(*referenceDoc); os.IsNotExist(err) {
		log.Fatalf("Error: reference document not found: %s", *referenceDoc)
	}

	// Устанавливаем env переменные для парсинга
	os.Setenv("REFERENCE_DOC", *referenceDoc)
	os.Setenv("DATA_DIR", *dataDir)

	// Загружаем .env (опционально)
	_ = godotenv.Load()

	// Загружаем конфиг
	cfg := config.Config{}
	if err := config.Init(&cfg); err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Вычисляем пути к файлам БД на основе имени документа

	// Создаём директорию для данных
	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		log.Fatalf("failed to create data directory: %v", err)
	}

	log.Printf("Reference document: %s", cfg.ReferenceDoc)
	log.Printf("Data directory: %s", cfg.DataDir)

	// Создаём app
	a, err := app.New(&cfg)
	if err != nil {
		log.Fatalf("failed to create app: %v", err)
	}

	// Устанавливаем путь для сохранения результатов (если указан)
	if *outputFile != "" {
		a.SetOutputPath(*outputFile)
	}

	// Инициализируем (проверка Ollama, загрузка БД)
	if err := a.Init(); err != nil {
		log.Fatalf("failed to initialize app: %v", err)
	}

	// Контекст с сигналами завершения
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	// Запускаем приложение
	if err := a.Run(ctx); err != nil {
		log.Fatalf("app stopped with error: %v", err)
	}
}
