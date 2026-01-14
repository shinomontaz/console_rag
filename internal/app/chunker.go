package app

import (
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

type Chunk struct {
	ID      string
	Text    string
	Source  string
	Section string
}

type Chunker struct {
	maxChunkSize int
	overlap      int
}

func NewChunker(maxChunkSize, overlap int) *Chunker {
	return &Chunker{
		maxChunkSize: maxChunkSize,
		overlap:      overlap,
	}
}

func (c *Chunker) ChunkMarkdown(content, source string) []Chunk {
	md := goldmark.New()
	reader := text.NewReader([]byte(content))
	doc := md.Parser().Parse(reader)

	var chunks []Chunk
	var currentChunk strings.Builder
	var currentSection string

	ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			if heading, ok := n.(*ast.Heading); ok {
				if currentChunk.Len() > 0 {
					chunks = append(chunks, c.createChunk(currentChunk.String(), source, currentSection))
					currentChunk.Reset()
				}

				headingText := extractText(heading, []byte(content))
				currentSection = headingText
				currentChunk.WriteString(headingText + "\n")
			} else if textNode, ok := n.(*ast.Text); ok {
				currentChunk.Write(textNode.Segment.Value([]byte(content)))
			} else if _, ok := n.(*ast.String); ok {
				currentChunk.WriteString(string(n.(*ast.String).Value))
			}
		} else {
			if _, ok := n.(*ast.Paragraph); ok {
				currentChunk.WriteString("\n")
			}
		}
		return ast.WalkContinue, nil
	})

	if currentChunk.Len() > 0 {
		chunks = append(chunks, c.createChunk(currentChunk.String(), source, currentSection))
	}

	return chunks
}

func extractText(node ast.Node, source []byte) string {
	var buf strings.Builder
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		if textNode, ok := child.(*ast.Text); ok {
			buf.Write(textNode.Segment.Value(source))
		}
	}
	return buf.String()
}

func (c *Chunker) ChunkText(content, source string) []Chunk {
	var chunks []Chunk
	runes := []rune(content)

	for i := 0; i < len(runes); i += c.maxChunkSize - c.overlap {
		end := i + c.maxChunkSize
		if end > len(runes) {
			end = len(runes)
		}

		chunkText := string(runes[i:end])
		chunks = append(chunks, c.createChunk(chunkText, source, ""))

		if end >= len(runes) {
			break
		}
	}

	return chunks
}

func (c *Chunker) createChunk(text, source, section string) Chunk {
	text = strings.TrimSpace(text)
	hash := sha256.Sum256([]byte(text + source))

	return Chunk{
		ID:      fmt.Sprintf("%x", hash[:8]),
		Text:    text,
		Source:  source,
		Section: section,
	}
}
