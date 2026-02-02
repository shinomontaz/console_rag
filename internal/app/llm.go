package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// queryLLM отправляет промпт в LLM и возвращает ответ
func (a *App) queryLLM(ctx context.Context, prompt string) (string, error) {
	// Формируем запрос в OpenAI-compatible формате
	reqBody := map[string]interface{}{
		"model": a.cfg.LlmMain.Model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"max_tokens":  a.cfg.MaxTokens,
		"temperature": a.cfg.Temperature,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Создаём HTTP запрос
	url := a.cfg.LlmMain.URL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if a.cfg.LlmMain.Key != "" {
		req.Header.Set("Authorization", "Bearer "+a.cfg.LlmMain.Key)
	}

	// Отправляем запрос
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("LLM returned status %d: %s", resp.StatusCode, string(body))
	}

	// Парсим ответ
	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no response from LLM")
	}

	return response.Choices[0].Message.Content, nil
}

// buildSimplePrompt формирует простой промпт без контроля размера
func buildSimplePrompt(
	inputText string,
	referenceResults []SearchResult,
) string {
	var buf strings.Builder

	buf.WriteString("Ты эксперт по сравнению юридических документов.\n\n")
	buf.WriteString("Analyzed text:\n<<<\n")
	buf.WriteString(inputText)
	buf.WriteString("\n>>>\n\n")

	buf.WriteString("Relevant sections from reference document:\n")
	for i, r := range referenceResults {
		buf.WriteString(fmt.Sprintf("%d. [Section: %s] (similarity: %.2f)\n", i+1, r.Section, r.Similarity))
		buf.WriteString("<<<\n")
		buf.WriteString(r.Content)
		buf.WriteString("\n>>>\n\n")
	}

	buf.WriteString("Task:\n")
	buf.WriteString("1. Сравни analyzed text с reference sections\n")
	buf.WriteString("2. Найди противоречия, несоответствия, риски\n")
	buf.WriteString("3. Оцени степень соответствия\n")
	buf.WriteString("4. Дай конкретные рекомендации\n\n")
	buf.WriteString("Output format:\n")
	buf.WriteString("- Status: [✅ Соответствует / ⚠️ Частичное соответствие / ❌ Противоречие]\n")
	buf.WriteString("- Issues: <список проблем>\n")
	buf.WriteString("- Recommendations: <рекомендации>\n")

	return buf.String()
}

// buildAnalysisPrompt формирует промпт с контролем размера для gemma3
func (a *App) buildAnalysisPrompt(
	inputText string,
	referenceResults []SearchResult,
) string {
	var buf strings.Builder

	systemPrompt := "Ты эксперт по сравнению юридических документов."
	buf.WriteString(systemPrompt)
	buf.WriteString("\n\nAnalyzed text:\n<<<\n")
	buf.WriteString(inputText)
	buf.WriteString("\n>>>\n\n")

	// Проверяем размер до добавления reference chunks
	currentSize := len(buf.String())
	availableForRefs := a.cfg.MaxPromptChars - currentSize - 500 // 500 для task description

	// Группируем reference chunks по секциям
	grouped := groupBySection(referenceResults)

	buf.WriteString("Relevant sections from reference document:\n")
	refCount := 0

	for section, chunks := range grouped {
		// Объединяем чанки из одной секции
		var combined strings.Builder
		for _, chunk := range chunks {
			combined.WriteString(chunk.Content)
			combined.WriteString("\n")
		}
		combinedText := combined.String()

		// Проверяем, влезет ли
		entrySize := len(section) + len(combinedText) + 100
		if currentSize+entrySize > availableForRefs {
			// Обрезаем текст
			maxTextLen := availableForRefs - currentSize - len(section) - 100
			if maxTextLen > 0 && maxTextLen < len(combinedText) {
				combinedText = combinedText[:maxTextLen] + "..."
			} else if maxTextLen <= 0 {
				break // Больше не влезает
			}
		}

		refCount++
		buf.WriteString(fmt.Sprintf("%d. [Section: %s] (similarity: %.2f)\n",
			refCount, section, chunks[0].Similarity))
		buf.WriteString("<<<\n")
		buf.WriteString(combinedText)
		buf.WriteString(">>>\n\n")

		currentSize += entrySize

		// Не добавляем слишком много секций
		if refCount >= 5 {
			break
		}
	}

	// Task description
	buf.WriteString("Task:\n")
	buf.WriteString("1. Сравни analyzed text с reference sections\n")
	buf.WriteString("2. Найди противоречия, несоответствия, риски\n")
	buf.WriteString("3. Оцени степень соответствия\n")
	buf.WriteString("4. Дай конкретные рекомендации\n\n")
	buf.WriteString("Output format:\n")
	buf.WriteString("- Status: [✅ Соответствует / ⚠️ Частичное соответствие / ❌ Противоречие]\n")
	buf.WriteString("- Issues: <список проблем>\n")
	buf.WriteString("- Recommendations: <рекомендации>\n")

	return buf.String()
}
