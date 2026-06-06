package rerank

import (
	"sort"

	"llmentor/rag-go/internal/document"
	"llmentor/rag-go/internal/es"
)

func RRF(vectorDocs []document.Document, keywordDocs []es.DocumentChunk, topK int) []string {
	if topK <= 0 {
		topK = 5
	}
	const k = 60.0
	scores := map[string]float64{}
	content := map[string]string{}

	for i, doc := range vectorDocs {
		rank := float64(i + 1)
		scores[doc.ID] += 1.0 / (k + rank)
		if _, ok := content[doc.ID]; !ok {
			content[doc.ID] = doc.Text
		}
	}
	for i, doc := range keywordDocs {
		rank := float64(i + 1)
		scores[doc.ID] += 1.0 / (k + rank)
		if _, ok := content[doc.ID]; !ok {
			content[doc.ID] = doc.Content
		}
	}

	ids := make([]string, 0, len(scores))
	for id := range scores {
		ids = append(ids, id)
	}
	sort.SliceStable(ids, func(i, j int) bool {
		return scores[ids[i]] > scores[ids[j]]
	})
	if topK > len(ids) {
		topK = len(ids)
	}
	out := make([]string, 0, topK)
	for _, id := range ids[:topK] {
		if content[id] != "" {
			out = append(out, content[id])
		}
	}
	return out
}
