package splitter

import (
	"strings"

	"llmentor/rag-go/internal/document"
)

func RecursiveDocuments(docs []document.Document, chunkSize, overlap int, separators []string) []document.Document {
	var out []document.Document
	for _, doc := range docs {
		chunks := RecursiveText(doc.Text, chunkSize, overlap, separators)
		for i, chunk := range chunks {
			out = append(out, document.WithChunk(doc, chunk, i))
		}
	}
	return out
}

func RecursiveText(text string, chunkSize, overlap int, separators []string) []string {
	if len([]rune(text)) <= chunkSize {
		if strings.TrimSpace(text) == "" {
			return nil
		}
		return []string{text}
	}
	for _, sep := range separators {
		if sep == "" || !strings.Contains(text, sep) {
			continue
		}
		parts := strings.Split(text, sep)
		var chunks []string
		var current strings.Builder
		for _, part := range parts {
			if strings.TrimSpace(part) == "" {
				continue
			}
			candidate := part
			if current.Len() > 0 {
				candidate = current.String() + sep + part
			}
			if len([]rune(candidate)) <= chunkSize {
				current.Reset()
				current.WriteString(candidate)
				continue
			}
			if current.Len() > 0 {
				chunks = append(chunks, current.String())
				current.Reset()
			}
			chunks = append(chunks, RecursiveText(part, chunkSize, overlap, separators[1:])...)
		}
		if current.Len() > 0 {
			chunks = append(chunks, current.String())
		}
		if len(chunks) > 0 {
			return chunks
		}
	}
	return OverlapText(text, chunkSize, overlap)
}
