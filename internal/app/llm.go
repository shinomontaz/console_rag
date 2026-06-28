package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	tiktoken "github.com/pkoukk/tiktoken-go"
	"google.golang.org/genai"
)

// Кэшированные объекты для производительности
var (
	reWhitespace = regexp.MustCompile(`\s+`)
	tiktokenOnce sync.Once
	tiktokenEnc  *tiktoken.Tiktoken
)

// countTokens точно подсчитывает количество токенов в тексте
// Использует cl100k_base encoding (для GPT-4, Qwen и совместимых моделей)
func countTokens(text string) int {
	tiktokenOnce.Do(func() {
		enc, err := tiktoken.GetEncoding("cl100k_base")
		if err != nil {
			// Fallback будет использоваться всегда
			return
		}
		tiktokenEnc = enc
	})
	if tiktokenEnc == nil {
		// Fallback на консервативную оценку для русского текста
		return len(text) / 2
	}
	tokens := tiktokenEnc.Encode(text, nil, nil)
	return len(tokens)
}

// queryLLM роутер для выбора провайдера LLM (с retry)
func (a *App) queryLLM(ctx context.Context, prompt string) (string, error) {
	// Логируем размер промпта для отладки
	tokenCount := countTokens(prompt)
	a.logger.Infof("📊 Prompt size: %d chars (%d tokens)", len(prompt), tokenCount)

	var result string
	err := a.withRetry(ctx, 3, func() error {
		var err error
		switch a.cfg.LlmMain.Type {
		case "gemini":
			result, err = a.queryGemini(ctx, prompt)
		case "openai":
			result, err = a.queryOpenAI(ctx, prompt)
		default:
			return fmt.Errorf("unknown LLM type: %s (supported: openai, gemini)", a.cfg.LlmMain.Type)
		}
		return err
	})
	return result, err
}

// withRetry выполняет fn с exponential backoff при ошибках
func (a *App) withRetry(ctx context.Context, maxRetries int, fn func() error) error {
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		lastErr = fn()
		if lastErr == nil {
			return nil
		}
		if attempt < maxRetries-1 {
			delay := time.Duration(1<<uint(attempt)) * time.Second
			a.logger.Infof("⏳ Retry %d/%d after %v: %v", attempt+1, maxRetries, delay, lastErr)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}
	}
	return fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}

// queryOpenAI отправляет промпт в OpenAI-compatible API (llama.cpp/qwen) и возвращает ответ
func (a *App) queryOpenAI(ctx context.Context, prompt string) (string, error) {
	reqBody := map[string]interface{}{
		//		"model": a.cfg.LlmMain.Model,
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

	url := a.cfg.LlmMain.URL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if a.cfg.LlmMain.Key != "" {
		req.Header.Set("Authorization", "Bearer "+a.cfg.LlmMain.Key)
	}

	resp, err := a.httpClient.Do(req) // использовали общего клиента
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		a.logger.Errorf("LLM error response: status=%d, body=%s", resp.StatusCode, string(body))

		return "", fmt.Errorf("LLM returned status %d: %s", resp.StatusCode, string(body))
	}

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
	resp, err := a.geminiClient.Models.GenerateContent(ctx, a.cfg.LlmMain.Model, genai.Text(prompt), &genai.GenerateContentConfig{
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

// cleanContentForPrompt - финальная очистка для промпта
func cleanContentForPrompt(content string) string {
	content = strings.TrimSpace(content)

	// Убрать обрезанные слова в начале
	if len(content) > 0 {
		r, _ := utf8.DecodeRuneInString(content)
		if unicode.IsLower(r) {
			removed := strings.Fields(content)[0]
			parts := strings.Fields(content)
			if len(parts) > 1 {
				content = strings.Join(parts[1:], " ")
				log.Printf("🔧 cleanContentForPrompt: removed leading word %q (starts with lowercase)", removed)
			}
		}
	}

	// Убрать обрезанные слова в конце
	content = strings.TrimRight(content, " -,")

	// Множественные пробелы → один пробел
	content = reWhitespace.ReplaceAllString(content, " ")

	return content
}

// buildAnalysisPrompt формирует промпт с контролем размера для gemma3
func (a *App) buildAnalysisPrompt(
	inputText string,
	referenceResults []SearchResult,
) string {
	var buf strings.Builder

	buf.WriteString(a.cfg.CustomPromt.Header)
	buf.WriteString("\n\n")

	buf.WriteString(a.cfg.CustomPromt.Chunk)
	buf.WriteString("\n")

	buf.WriteString(strings.TrimSpace(inputText))
	buf.WriteString("\n\n")

	// Эталон (максимум 3 чанка)
	buf.WriteString(a.cfg.CustomPromt.Etalon)
	buf.WriteString("\n")

	addedCount := 0
	for _, result := range referenceResults {
		if addedCount >= 3 {
			break
		}

		cleanContent := cleanContentForPrompt(result.Content)

		// Пропускаем слишком короткие
		if len(cleanContent) < 30 {
			continue
		}

		buf.WriteString(fmt.Sprintf("%d. %s\n", addedCount+1, cleanContent))
		addedCount++
	}

	if addedCount == 0 {
		buf.WriteString("(нет релевантных разделов)\n")
	}

	buf.WriteString("\n")

	buf.WriteString(a.cfg.CustomPromt.Footer)
	buf.WriteString("\n")

	return buf.String()
}
