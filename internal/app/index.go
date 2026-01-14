package app

func (a *App) indexDocument() error {
	// ctx := context.Background()

	// // Get existing collection or create new one
	// coll := a.db.GetCollection("docs", a.embeddingFunc)
	// if coll == nil {
	// 	var err error
	// 	coll, err = a.db.CreateCollection("docs", map[string]string{}, a.embeddingFunc)
	// 	if err != nil {
	// 		return fmt.Errorf("failed to create collection: %w", err)
	// 	}
	// }

	// // If force-reindex is set, clear everything
	// if a.cfg.ForceReindex {
	// 	log.Printf("Force reindexing enabled, clearing existing metadata and collection")
	// 	a.metadata.Files = make(map[string]FileInfo)
	// 	// Remove and recreate collection
	// 	a.db.DeleteCollection("docs")
	// 	coll, _ = a.db.CreateCollection("docs", map[string]string{}, a.embeddingFunc)
	// }

	// log.Printf("Current metadata contains %d files", len(a.metadata.Files))
	// log.Printf("Indexing documents in: %s", a.cfg.DocsDir)

	// a.metadata.DataPath = a.cfg.DocsDir
	// // Walk through docs directory
	// err := filepath.Walk(a.cfg.DocsDir, func(path string, info os.FileInfo, err error) error {
	// 	log.Printf("Walking path: %s", path)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	// Skip directories and non-text files
	// 	if info.IsDir() || !fileCanProcess(path) {
	// 		log.Printf("Skipping inacceptable file: %s", path)
	// 		return nil
	// 	}

	// 	// Check if file needs indexing
	// 	relPath, _ := filepath.Rel(a.cfg.DocsDir, path)
	// 	fileInfo, exists := a.metadata.Files[relPath]
	// 	if !a.cfg.ForceReindex && exists && fileInfo.LastModified.Equal(info.ModTime()) && fileInfo.Size == info.Size() {
	// 		log.Printf("Skipping unchanged file: %s", relPath)
	// 		return nil
	// 	}
	// 	log.Printf("Indexing file: %s", relPath)

	// 	//		content := fileGetContent(path)

	// 	// Split content into chunks
	// 	// chunks := a.chunker.Chunk(content)
	// 	// for _, ch := range chunks {
	// 	// 	doc := chromem.Document{
	// 	// 		ID:      ch.ID,
	// 	// 		Content: ch.Text,
	// 	// 		Metadata: map[string]string{
	// 	// 			"source":  ch.Source,
	// 	// 			"article": ch.Article,
	// 	// 		},
	// 	// 	}
	// 	// 	coll.AddDocuments(ctx, []chromem.Document{doc}, runtime.NumCPU())
	// 	// }

	// 	// Update metadata (store only for the file, not per chunk)
	// 	a.metadata.Files[relPath] = FileInfo{
	// 		Path:         relPath,
	// 		LastModified: info.ModTime(),
	// 		Size:         info.Size(),
	// 	}

	// 	//		log.Printf("Indexed file: %s (%d chunks)", relPath, len(chunks))
	// 	return nil
	// })

	// if err != nil {
	// 	return fmt.Errorf("failed to walk docs directory: %w", err)
	// }

	// // Save metadata and DB
	// if err := a.saveMetadata(); err != nil {
	// 	return fmt.Errorf("failed to save metadata: %w", err)
	// }

	// if err := a.saveDB(); err != nil {
	// 	return fmt.Errorf("failed to save vector database: %w", err)
	// }

	return nil
}

func fileCanProcess(path string) bool {
	// TODO: проверяем, что данный файл это .md, txt или pdf
	return false
}

func fileGetContent(path string) string {
	// TODO: берем текст из txt или pdf файла
	return ""
}
