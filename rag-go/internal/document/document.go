package document

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

type Document struct {
	ID       string         `json:"id"`
	Text     string         `json:"text"`
	Metadata map[string]any `json:"metadata,omitempty"`
	Score    float64        `json:"score,omitempty"`
}

func New(text string, metadata map[string]any) Document {
	if metadata == nil {
		metadata = map[string]any{}
	}
	return Document{
		ID:       NewID(),
		Text:     text,
		Metadata: metadata,
	}
}

func NewID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	dst := make([]byte, 36)
	hex.Encode(dst[0:8], b[0:4])
	dst[8] = '-'
	hex.Encode(dst[9:13], b[4:6])
	dst[13] = '-'
	hex.Encode(dst[14:18], b[6:8])
	dst[18] = '-'
	hex.Encode(dst[19:23], b[8:10])
	dst[23] = '-'
	hex.Encode(dst[24:], b[10:])
	return string(dst)
}

func CloneMetadata(src map[string]any) map[string]any {
	dst := map[string]any{}
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func WithChunk(parent Document, text string, index int) Document {
	meta := CloneMetadata(parent.Metadata)
	meta["parentId"] = parent.ID
	meta["chunkIndex"] = index
	id := NewID()
	meta["chunkId"] = fmt.Sprintf("%s:%d", parent.ID, index)
	return Document{ID: id, Text: text, Metadata: meta}
}
