package cleaner

import (
	"regexp"
	"strings"

	"llmentor/rag-go/internal/document"
)

var (
	spaceRE   = regexp.MustCompile(`\s+`)
	invalidRE = regexp.MustCompile(`[^\pL\pN\pP\pZ\n]+`)
	newlineRE = regexp.MustCompile(`\n+`)
)

func CleanDocuments(docs []document.Document) []document.Document {
	out := make([]document.Document, 0, len(docs))
	for _, doc := range docs {
		text := CleanText(doc.Text)
		if text == "" {
			continue
		}
		doc.Text = text
		out = append(out, doc)
	}
	return out
}

func CleanText(text string) string {
	text = strings.TrimSpace(spaceRE.ReplaceAllString(text, " "))
	text = invalidRE.ReplaceAllString(text, "")

	seen := map[string]struct{}{}
	parts := newlineRE.Split(text, -1)
	kept := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if _, ok := seen[part]; ok {
			continue
		}
		seen[part] = struct{}{}
		kept = append(kept, part)
	}
	return strings.Join(kept, "\n")
}
