package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"console_rag/internal/chunker"
	"console_rag/internal/config"

	"github.com/ledongthuc/pdf"
	"github.com/philippgille/chromem-go"
)

type App struct {
	cfg            *config.Config
	db             *chromem.DB
	metadata       *Metadata
	fileDB         string
	fileMetadata   string
	embeddingFunc  chromem.EmbeddingFunc
	chunkerFactory *chunker.Factory
	outputPath     string
}

type Metadata struct {
	Files    map[string]FileInfo `json:"files"`
	DataPath string              `json:"data_path"`
}

type FileInfo struct {
	Path         string    `json:"path"`
	LastModified time.Time `json:"last_modified"`
	Size         int64     `json:"size"`
}

func New(cfg *config.Config) (*App, error) {
	// Создаём фабрику chunker'ов
	chunkerConfig := chunker.Config{
		MaxChunkSize: cfg.ChunkSize,
		Overlap:      cfg.ChunkOverlap,
	}

	app := &App{
		cfg:            cfg,
		metadata:       &Metadata{Files: make(map[string]FileInfo)},
		chunkerFactory: chunker.NewFactory(chunkerConfig),
	}

	docBaseName := strings.TrimSuffix(filepath.Base(cfg.ReferenceDoc), filepath.Ext(cfg.ReferenceDoc))
	app.fileMetadata = filepath.Join(cfg.DataDir, docBaseName+"_metadata.json")
	app.fileDB = filepath.Join(cfg.DataDir, docBaseName+".gob")
	log.Printf("DB file: %s", app.fileDB)
	log.Printf("Metadata file: %s", app.fileMetadata)

	normalized := true
	app.embeddingFunc = chromem.NewEmbeddingFuncOpenAICompat(cfg.LlmEmbed.URL, cfg.LlmEmbed.Key, cfg.LlmEmbed.Model, &normalized)

	app.db = chromem.NewDB()

	return app, nil
}

func (a *App) Init() error {
	fileInfo, err := os.Stat(a.cfg.ReferenceDoc)
	if err != nil {
		return fmt.Errorf("reference document not found: %w", err)
	}

	// Check if DB exists for this document
	if _, err := os.Stat(a.fileDB); err == nil {
		log.Printf("💾 Found existing DB, loading...")
		if err := a.loadDB(); err != nil {
			return fmt.Errorf("failed to load database: %w", err)
		}
		_ = a.loadMetadata()
		log.Printf("✅ Database loaded")

		return nil
	}

	log.Printf("📚 No DB found, indexing document...")
	if err := a.indexDocument(fileInfo); err != nil {
		return fmt.Errorf("failed to index document: %w", err)
	}

	return nil
}

func (a *App) indexDocument(fileInfo os.FileInfo) error {
	ctx := context.Background()

	content, err := readFile(a.cfg.ReferenceDoc)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	log.Printf("📄 File size: %d bytes", len(content))

	chunkr, err := a.chunkerFactory.GetChunker(a.cfg.ReferenceDoc, a.cfg.ChunkMethod)
	if err != nil {
		return fmt.Errorf("failed to get chunker: %w", err)
	}

	log.Printf("🔧 Using chunker: %s", chunkr.Name())

	chunks, err := chunkr.Chunk(content, filepath.Base(a.cfg.ReferenceDoc))
	if err != nil {
		log.Printf("⚠️  Markdown chunker failed: %v", err)
		log.Printf("🔄 Falling back to text chunker...")

		textChunker := chunker.NewTextChunker(chunker.Config{
			MaxChunkSize: a.cfg.ChunkSize,
			Overlap:      a.cfg.ChunkOverlap,
		})

		chunks, err = textChunker.Chunk(content, filepath.Base(a.cfg.ReferenceDoc))
		if err != nil {
			return fmt.Errorf("text chunker also failed: %w", err)
		}

		log.Printf("✅ Text chunker succeeded")
	}

	if len(chunks) == 0 {
		return fmt.Errorf("no chunks created from document")
	}

	log.Printf("📦 Created %d chunks", len(chunks))

	coll := a.db.GetCollection("docs", a.embeddingFunc)
	if coll == nil {
		coll, err = a.db.CreateCollection("docs", nil, a.embeddingFunc)
		if err != nil {
			return fmt.Errorf("failed to create collection: %w", err)
		}
	}

	log.Printf("🔄 Adding chunks to vector database...")
	successCount := 0
	for i, chunk := range chunks {
		doc := chromem.Document{
			ID:       chunk.ID,
			Content:  chunk.Text,
			Metadata: chunk.Metadata,
		}

		if doc.Metadata == nil {
			doc.Metadata = make(map[string]string)
		}
		doc.Metadata["source"] = chunk.Source
		doc.Metadata["section"] = chunk.Section

		err := coll.AddDocument(ctx, doc)
		if err != nil {
			log.Printf("⚠️  Failed to add chunk %d (%s): %v", i+1, chunk.ID, err)
		} else {
			successCount++
		}

		if (i+1)%5 == 0 || i+1 == len(chunks) {
			log.Printf("   Progress: %d/%d chunks added", successCount, i+1)
		}

		// Rate limiting: ~10 req/s to respect nginx limits
		if i < len(chunks)-1 {
			time.Sleep(150 * time.Millisecond)
		}
	}

	if successCount == 0 {
		return fmt.Errorf("failed to add any chunks to database")
	}

	log.Printf("✅ Successfully added %d/%d chunks to vector database", successCount, len(chunks))

	relPath := filepath.Base(a.cfg.ReferenceDoc)
	a.metadata.Files[relPath] = FileInfo{
		Path:         relPath,
		LastModified: fileInfo.ModTime(),
		Size:         fileInfo.Size(),
	}

	if err := a.saveMetadata(); err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	log.Printf("💾 Saving vector database...")
	if err := a.saveDB(); err != nil {
		return fmt.Errorf("failed to save database: %w", err)
	}

	log.Printf("✅ Reference document indexed successfully")
	return nil
}

func readFile(path string) (string, error) {
	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".md", ".markdown", ".txt":
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		return string(data), nil
	case ".pdf":
		return readPDF(path)
	default:
		return "", fmt.Errorf("unsupported file format: %s", ext)
	}
}

func readPDF(path string) (string, error) {
	f, r, err := pdf.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open PDF: %w", err)
	}
	defer f.Close()

	var textBuilder strings.Builder
	totalPages := r.NumPage()

	for pageNum := 1; pageNum <= totalPages; pageNum++ {
		page := r.Page(pageNum)
		if page.V.IsNull() {
			continue
		}

		text, err := page.GetPlainText(nil)
		if err != nil {
			log.Printf("Warning: failed to extract text from page %d: %v", pageNum, err)
			continue
		}

		textBuilder.WriteString(text)
		textBuilder.WriteString("\n\n")
	}

	result := textBuilder.String()
	if len(result) == 0 {
		return "", fmt.Errorf("no text extracted from PDF")
	}

	return result, nil
}

// Helper to print address nicely in logs
func trimHostPrefix(addr string) string {
	if addr == "" {
		return "localhost"
	}
	if addr[0] == ':' {
		return "127.0.0.1" + addr
	}
	return addr
}

func (a *App) loadMetadata() error {
	f, err := os.Open(a.fileMetadata)
	if os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	defer f.Close()

	return json.NewDecoder(f).Decode(&a.metadata)
}

func (a *App) saveMetadata() error {
	f, err := os.Create(a.fileMetadata)
	if err != nil {
		return err
	}
	defer f.Close()

	return json.NewEncoder(f).Encode(a.metadata)
}

func (a *App) loadDB() error {
	log.Printf("Loading vector database from: %s", a.fileDB)
	err := a.db.ImportFromFile(a.fileDB, "", "docs")
	if err != nil {
		return fmt.Errorf("failed to import DB: %w", err)
	}

	// Проверяем состояние после загрузки
	coll := a.db.GetCollection("docs", a.embeddingFunc)
	if coll == nil {
		log.Printf("Warning: Collection 'docs' not found after DB load")
	} else {
		log.Printf("Successfully loaded vector database and found 'docs' collection")
	}

	return nil
}

func (a *App) saveDB() error {
	return a.db.ExportToFile(a.fileDB, true, "", "docs")
}

func (a *App) SetOutputPath(path string) {
	a.outputPath = path
}
