package splitter

import (
	"strings"
	"unicode"
)

func SentenceText(text string, chunkSize int) []string {
	if chunkSize <= 0 {
		chunkSize = 100
	}
	var sentences []string
	start := 0
	runes := []rune(text)
	for i, r := range runes {
		if r == '.' || r == '?' || r == '!' || r == '\u3002' || r == '\uff1f' || r == '\uff01' {
			end := i + 1
			s := strings.TrimSpace(string(runes[start:end]))
			if s != "" {
				sentences = append(sentences, s)
			}
			start = end
		}
	}
	if start < len(runes) {
		s := strings.TrimSpace(string(runes[start:]))
		if s != "" {
			sentences = append(sentences, s)
		}
	}

	var chunks []string
	var current []rune
	for _, s := range sentences {
		sr := []rune(s)
		if len(current)+len(sr) > chunkSize && len(current) > 0 {
			chunks = append(chunks, strings.TrimSpace(string(current)))
			current = current[:0]
		}
		if len(current) > 0 && !unicode.IsSpace(current[len(current)-1]) {
			current = append(current, ' ')
		}
		current = append(current, sr...)
	}
	if len(current) > 0 {
		chunks = append(chunks, strings.TrimSpace(string(current)))
	}
	return chunks
}
