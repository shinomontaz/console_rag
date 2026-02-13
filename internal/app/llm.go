package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	tiktoken "github.com/pkoukk/tiktoken-go"
	"google.golang.org/genai"
)

// countTokens точно подсчитывает количество токенов в тексте
// Использует cl100k_base encoding (для GPT-4, Qwen и совместимых моделей)
func countTokens(text string) int {
	encoding, err := tiktoken.GetEncoding("cl100k_base")
	if err != nil {
		// Fallback на консервативную оценку для русского текста
		return len(text) / 2
	}
	tokens := encoding.Encode(text, nil, nil)
	return len(tokens)
}

// queryLLM роутер для выбора провайдера LLM
func (a *App) queryLLM(ctx context.Context, prompt string) (string, error) {
	// Логируем размер промпта для отладки
	tokenCount := countTokens(prompt)
	log.Printf("📊 Prompt size: %d chars (%d tokens)", len(prompt), tokenCount)

	// Выбор провайдера по типу
	switch a.cfg.LlmMain.Type {
	case "gemini":
		return a.queryGemini(ctx, prompt)
	case "openai":
		return a.queryOpenAI(ctx, prompt)
	default:
		return "", fmt.Errorf("unknown LLM type: %s (supported: openai, gemini)", a.cfg.LlmMain.Type)
	}
}

// queryOpenAI отправляет промпт в OpenAI-compatible API (llama.cpp/qwen) и возвращает ответ
func (a *App) queryOpenAI(ctx context.Context, prompt string) (string, error) {
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
	client := &http.Client{
		Timeout: 5 * time.Minute, // Для больших промптов
	}
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

// queryGemini отправляет промпт в Gemini API и возвращает ответ
func (a *App) queryGemini(ctx context.Context, prompt string) (string, error) {
	// Создаём Gemini клиент
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  a.cfg.LlmMain.Key,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create Gemini client: %w", err)
	}

	// Генерируем контент
	resp, err := client.Models.GenerateContent(ctx, a.cfg.LlmMain.Model, genai.Text(prompt), &genai.GenerateContentConfig{
		Temperature:     &a.cfg.Temperature,
		MaxOutputTokens: int32(a.cfg.MaxTokens),
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate content: %w", err)
	}

	if len(resp.Candidates) == 0 {
		return "", fmt.Errorf("no response from Gemini")
	}

	candidate := resp.Candidates[0]
	if candidate.Content == nil {
		return "", fmt.Errorf("no content in Gemini response")
	}

	// Извлекаем текст из ответа
	var responseText strings.Builder
	for _, part := range candidate.Content.Parts {
		if part.Text != "" {
			responseText.WriteString(part.Text)
		}
	}

	if responseText.Len() == 0 {
		return "", fmt.Errorf("empty response from Gemini")
	}

	return strings.TrimSpace(responseText.String()), nil
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

	// Проверяем размер до добавления reference chunks (в токенах)
	currentTokens := countTokens(buf.String())
	availableTokens := 3500 // 4096 context - 500 для response - 96 запас

	// Группируем reference chunks по секциям
	grouped := groupBySection(referenceResults)

	buf.WriteString("Relevant sections from reference document:\n")
	refCount := 0

	for section, chunks := range grouped {
		var combined strings.Builder
		for _, chunk := range chunks {
			combined.WriteString(chunk.Content)
			combined.WriteString("\n")
		}
		combinedText := combined.String()

		// Проверяем, влезет ли (в токенах)
		entryText := fmt.Sprintf("%d. [Section: %s]\n%s\n", refCount+1, section, combinedText)
		entryTokens := countTokens(entryText)
		if currentTokens+entryTokens > availableTokens {
			// Обрезаем текст по токенам
			maxTextTokens := availableTokens - currentTokens - countTokens(section) - 50
			if maxTextTokens > 0 {
				// Итеративно обрезаем до нужного размера токенов
				for countTokens(combinedText) > maxTextTokens && len(combinedText) > 0 {
					combinedText = combinedText[:len(combinedText)*9/10] // Обрезаем по 10%
				}
				combinedText += "..."
				entryText = fmt.Sprintf("%d. [Section: %s]\n%s\n", refCount+1, section, combinedText)
				entryTokens = countTokens(entryText)
			} else {
				break // Больше не влезает
			}
		}

		refCount++
		buf.WriteString(fmt.Sprintf("%d. [Section: %s] (similarity: %.2f)\n",
			refCount, section, chunks[0].Similarity))
		buf.WriteString("<<<\n")
		buf.WriteString(combinedText)
		buf.WriteString(">>>\n\n")

		currentTokens += entryTokens

		if refCount >= 3 {
			break
		}
	}

	buf.WriteString("\nCompare texts. Find contradictions and risks. Output:\nStatus: ✅/⚠️/❌\nIssues: ...\nRecommendations: ...\n")

	return buf.String()
}
