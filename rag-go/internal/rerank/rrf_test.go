package rerank

import (
	"testing"

	"llmentor/rag-go/internal/document"
	"llmentor/rag-go/internal/es"
)

func TestRRF(t *testing.T) {
	vector := []document.Document{{ID: "a", Text: "A"}, {ID: "b", Text: "B"}}
	keyword := []es.DocumentChunk{{ID: "b", Content: "B"}, {ID: "c", Content: "C"}}
	got := RRF(vector, keyword, 2)
	if len(got) != 2 {
		t.Fatalf("len=%d", len(got))
	}
	if got[0] != "B" {
		t.Fatalf("first=%q want B", got[0])
	}
}
