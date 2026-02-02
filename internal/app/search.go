package app

import (
	"context"
	"fmt"
)

// SearchResult - результат векторного поиска
type SearchResult struct {
	Content    string
	Section    string
	Source     string
	Similarity float32
}

// searchRelevantChunks ищет релевантные чанки в reference-doc
func (a *App) searchRelevantChunks(
	ctx context.Context,
	queryText string,
) ([]SearchResult, error) {
	// Получаем коллекцию
	coll := a.db.GetCollection("docs", a.embeddingFunc)
	if coll == nil {
		return nil, fmt.Errorf("collection 'docs' not found")
	}

	// Выполняем поиск
	results, err := coll.Query(ctx, queryText, a.cfg.TopK, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	// Фильтруем по similarity и преобразуем
	var searchResults []SearchResult
	for _, r := range results {
		if r.Similarity < a.cfg.MinSimilarity {
			continue
		}

		searchResults = append(searchResults, SearchResult{
			Content:    r.Content,
			Section:    r.Metadata["section"],
			Source:     r.Metadata["source"],
			Similarity: r.Similarity,
		})
	}

	return searchResults, nil
}

// groupBySection группирует результаты по секциям
func groupBySection(results []SearchResult) map[string][]SearchResult {
	grouped := make(map[string][]SearchResult)
	for _, r := range results {
		section := r.Section
		if section == "" {
			section = "Unknown"
		}
		grouped[section] = append(grouped[section], r)
	}
	return grouped
}
