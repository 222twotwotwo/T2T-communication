package splitter

import (
	"llmentor/rag-go/internal/document"
)

func OverlapDocuments(docs []document.Document, chunkSize, overlap int) []document.Document {
	var out []document.Document
	for _, doc := range docs {
		chunks := OverlapText(doc.Text, chunkSize, overlap)
		for i, chunk := range chunks {
			out = append(out, document.WithChunk(doc, chunk, i))
		}
	}
	return out
}

func OverlapText(text string, chunkSize, overlap int) []string {
	if chunkSize <= 0 {
		chunkSize = 100
	}
	if overlap < 0 {
		overlap = 0
	}
	if overlap >= chunkSize {
		overlap = chunkSize / 5
	}
	runes := []rune(text)
	if len(runes) == 0 {
		return nil
	}

	var chunks []string
	start := 0
	for start < len(runes) {
		end := start + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[start:end]))
		if end == len(runes) {
			break
		}
		start = end - overlap
		if start < 0 {
			start = 0
		}
	}
	return chunks
}
