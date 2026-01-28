package chunker

// Chunk представляет единицу текста для векторизации
type Chunk struct {
	ID       string            // Уникальный идентификатор (hash)
	Text     string            // Текст чанка
	Source   string            // Имя исходного файла
	Section  string            // Название секции (заголовок, глава и т.д.)
	Metadata map[string]string // Дополнительные метаданные
}

// Chunker - интерфейс для всех типов chunker'ов
type Chunker interface {
	// Chunk разбивает контент на чанки
	Chunk(content, source string) ([]Chunk, error)
	
	// Name возвращает название chunker'а для логирования
	Name() string
}

// Config содержит общие параметры для chunker'ов
type Config struct {
	MaxChunkSize int // Максимальный размер чанка в символах
	Overlap      int // Размер overlap между чанками
}
