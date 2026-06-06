package splitter

import (
	"strings"

	"llmentor/rag-go/internal/document"
)

func MarkdownHeaderDocuments(text string) []document.Document {
	lines := strings.Split(text, "\n")
	var current []string
	meta := map[string]any{}
	var out []document.Document

	flush := func() {
		body := strings.TrimSpace(strings.Join(current, "\n"))
		if body == "" {
			current = nil
			return
		}
		out = append(out, document.New(body, document.CloneMetadata(meta)))
		current = nil
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			flush()
			level := strings.Count(strings.Split(trimmed, " ")[0], "#")
			title := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
			meta["h"+string(rune('0'+level))] = title
			continue
		}
		current = append(current, line)
	}
	flush()
	return out
}
