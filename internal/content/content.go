package content

import (
	"strings"
)

type BlockType string

const (
	BlockParagraph BlockType = "paragraph"
	BlockHeading   BlockType = "heading"
	BlockCode      BlockType = "code"
	BlockListItem  BlockType = "list_item"
)

type Block struct {
	Type BlockType `json:"type"`
	Text string    `json:"text"`
}

type SourceDocument struct {
	Title    string          `json:"title"`
	Author   string          `json:"author,omitempty"`
	Language string          `json:"language,omitempty"`
	Cover    string          `json:"cover,omitempty"`
	Sections []SourceSection `json:"sections"`
}

type SourceSection struct {
	Title string `json:"title"`
	Start int    `json:"start"`
	End   int    `json:"end"`
	Text  string `json:"text"`
}

type Chunk struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Start  int    `json:"start"`
	End    int    `json:"end"`
	Text   string `json:"text"`
}

type TranslatedChunk struct {
	Number int     `json:"number"`
	Title  string  `json:"title"`
	Start  int     `json:"start"`
	End    int     `json:"end"`
	Blocks []Block `json:"blocks"`
}

type TranslatedDocument struct {
	Title    string            `json:"title"`
	Author   string            `json:"author,omitempty"`
	Language string            `json:"language"`
	Cover    string            `json:"cover,omitempty"`
	Chapters []TranslatedChunk `json:"chapters"`
}

func PlanChunks(doc SourceDocument, maxChars int) []Chunk {
	if maxChars <= 0 {
		maxChars = 6000
	}

	var chunks []Chunk
	for _, section := range doc.Sections {
		text := strings.TrimSpace(section.Text)
		if text == "" {
			continue
		}
		for _, part := range splitText(text, maxChars) {
			chunks = append(chunks, Chunk{
				Number: len(chunks) + 1,
				Title:  section.Title,
				Start:  section.Start,
				End:    section.End,
				Text:   part,
			})
		}
	}
	return chunks
}

func BlocksFromPlainText(text string) []Block {
	var blocks []Block
	var paragraph []string
	var code []string
	inCode := false

	flushParagraph := func() {
		if len(paragraph) == 0 {
			return
		}
		joined := strings.TrimSpace(strings.Join(paragraph, "\n"))
		if joined != "" {
			blocks = append(blocks, Block{Type: BlockParagraph, Text: joined})
		}
		paragraph = nil
	}
	flushCode := func() {
		joined := strings.TrimRight(strings.Join(code, "\n"), "\n")
		blocks = append(blocks, Block{Type: BlockCode, Text: joined})
		code = nil
	}

	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			if inCode {
				flushCode()
				inCode = false
				continue
			}
			flushParagraph()
			inCode = true
			continue
		}
		if inCode {
			code = append(code, line)
			continue
		}
		if trimmed == "" {
			flushParagraph()
			continue
		}
		paragraph = append(paragraph, line)
	}
	if inCode {
		flushCode()
	}
	flushParagraph()

	return blocks
}

func splitText(text string, maxChars int) []string {
	if len(text) <= maxChars {
		return []string{text}
	}

	paragraphs := splitParagraphs(text)
	var parts []string
	var current strings.Builder
	for _, paragraph := range paragraphs {
		if current.Len() > 0 && current.Len()+len(paragraph)+2 > maxChars {
			parts = append(parts, strings.TrimSpace(current.String()))
			current.Reset()
		}
		if len(paragraph) > maxChars {
			if current.Len() > 0 {
				parts = append(parts, strings.TrimSpace(current.String()))
				current.Reset()
			}
			parts = append(parts, splitLongParagraph(paragraph, maxChars)...)
			continue
		}
		if current.Len() > 0 {
			current.WriteString("\n\n")
		}
		current.WriteString(paragraph)
	}
	if current.Len() > 0 {
		parts = append(parts, strings.TrimSpace(current.String()))
	}
	return parts
}

func splitParagraphs(text string) []string {
	raw := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n\n")
	paragraphs := make([]string, 0, len(raw))
	for _, paragraph := range raw {
		paragraph = strings.TrimSpace(paragraph)
		if paragraph != "" {
			paragraphs = append(paragraphs, paragraph)
		}
	}
	return paragraphs
}

func splitLongParagraph(text string, maxChars int) []string {
	words := strings.Fields(text)
	var parts []string
	var current strings.Builder
	for _, word := range words {
		if current.Len() > 0 && current.Len()+len(word)+1 > maxChars {
			parts = append(parts, strings.TrimSpace(current.String()))
			current.Reset()
		}
		if current.Len() > 0 {
			current.WriteByte(' ')
		}
		current.WriteString(word)
	}
	if current.Len() > 0 {
		parts = append(parts, strings.TrimSpace(current.String()))
	}
	return parts
}
