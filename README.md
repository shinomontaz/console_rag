# Console RAG

Консольная утилита для анализа текстов на соответствие эталонному документу с использованием RAG (Retrieval-Augmented Generation).

## Основной use case

Анализ юридических документов, договоров и актов на соответствие другому документу (Трудовой кодекс, Налоговый кодекс и т.д.).

## Возможности

- ✅ Векторизация и индексация reference document
- ✅ Адаптивный chunking для Markdown (автоопределение структуры)
- ✅ Simple chunking для plain text с overlap
- ✅ Поддержка форматов: `.md`, `.txt`, `.pdf`
- ✅ Интеграция с OpenAI-совместимыми API (llama.cpp, qwen) и Google Gemini
- ✅ Персистентная векторная БД (chromem-go)
- ✅ Настраиваемые промпты для анализа
- ✅ Параллельная обработка больших документов
## Быстрый старт

### Скачать релиз

1. Перейдите в [Releases](https://github.com/shinomontaz/console_rag/releases)
2. Скачайте бинарник для вашей ОС
3. Скачайте [example.env](example.env)

### Настройка

1. Переименуйте [example.env](example.env) в `.env`:
```bash
cp example.env .env
```

2. Отредактируйте .env - укажите ваши API ключи и URL:

```
# LLM для генерации ответов (OpenAI-совместимый API)
LLM_MAIN_URL=https://your-api.com
LLM_MAIN_MODEL=qwen2.5-3b-instruct
LLM_MAIN_KEY=your-key-here
LLM_MAIN_TYPE=openai
 
# LLM для embeddings
LLM_EMBED_URL=https://your-embed-api.com/v1
LLM_EMBED_MODEL=nomic-embed-text
LLM_EMBED_KEY=your-embed-key
```