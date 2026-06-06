package reader

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"html"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/ledongthuc/pdf"

	"llmentor/rag-go/internal/document"
)

type Factory struct {
	TikaURL string
	Client  *http.Client
}

func NewFactory(tikaURL string) *Factory {
	return &Factory{
		TikaURL: strings.TrimRight(tikaURL, "/"),
		Client:  &http.Client{Timeout: 2 * time.Minute},
	}
}

func (f *Factory) Read(path string) ([]document.Document, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".txt", ".text", ".log":
		return f.readPlain(path, "text")
	case ".md", ".markdown":
		return f.readMarkdown(path)
	case ".html", ".htm":
		return f.readHTML(path)
	case ".json":
		return f.readJSON(path)
	case ".pdf":
		return f.readPDF(path)
	case ".doc", ".docx":
		if f.TikaURL != "" {
			return f.readWithTika(path)
		}
		if ext == ".docx" {
			return f.readDOCX(path)
		}
		return nil, errors.New("doc files require reader.tika_url")
	default:
		return nil, errors.New("unsupported file type: " + filepath.Base(path))
	}
}

func (f *Factory) readPlain(path, typ string) ([]document.Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return []document.Document{document.New(string(data), metadata(path, typ))}, nil
}

func (f *Factory) readMarkdown(path string) ([]document.Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	text := stripMarkdownCodeFences(string(data))
	return []document.Document{document.New(text, metadata(path, "markdown"))}, nil
}

func (f *Factory) readHTML(path string) ([]document.Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	text := stripHTML(string(data))
	return []document.Document{document.New(text, metadata(path, "html"))}, nil
}

func (f *Factory) readJSON(path string) ([]document.Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var value any
	if err := json.Unmarshal(data, &value); err == nil {
		if pretty, err := json.MarshalIndent(value, "", "  "); err == nil {
			data = pretty
		}
	}
	return []document.Document{document.New(string(data), metadata(path, "json"))}, nil
}

func (f *Factory) readPDF(path string) ([]document.Document, error) {
	file, reader, err := pdf.Open(path)
	if err != nil {
		if f.TikaURL != "" {
			return f.readWithTika(path)
		}
		return nil, err
	}
	defer file.Close()

	var docs []document.Document
	for pageNum := 1; pageNum <= reader.NumPage(); pageNum++ {
		page := reader.Page(pageNum)
		if page.V.IsNull() {
			continue
		}
		text, err := page.GetPlainText(nil)
		if err != nil {
			return nil, err
		}
		meta := metadata(path, "pdf")
		meta["page"] = pageNum
		docs = append(docs, document.New(text, meta))
	}
	return docs, nil
}

func (f *Factory) readDOCX(path string) ([]document.Document, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	for _, file := range r.File {
		if file.Name != "word/document.xml" {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		data, err := io.ReadAll(rc)
		if err != nil {
			return nil, err
		}
		text := docxXMLToText(data)
		return []document.Document{document.New(text, metadata(path, "docx"))}, nil
	}
	return nil, errors.New("word/document.xml not found in docx")
}

func (f *Factory) readWithTika(path string) ([]document.Document, error) {
	body, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, f.TikaURL+"/tika", body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/plain")
	resp, err := f.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, errors.New("tika request failed: " + resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return []document.Document{document.New(string(data), metadata(path, "tika"))}, nil
}

func metadata(path, typ string) map[string]any {
	return map[string]any{
		"source":   path,
		"filename": filepath.Base(path),
		"type":     typ,
	}
}

func stripMarkdownCodeFences(text string) string {
	var out []string
	inFence := false
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

var (
	scriptStyleRE = regexp.MustCompile(`(?is)<(script|style)[^>]*>.*?</(script|style)>`)
	tagRE         = regexp.MustCompile(`(?s)<[^>]+>`)
	wsRE          = regexp.MustCompile(`[ \t\r\f\v]+`)
)

func stripHTML(text string) string {
	text = scriptStyleRE.ReplaceAllString(text, " ")
	text = strings.ReplaceAll(text, "<br>", "\n")
	text = strings.ReplaceAll(text, "<br/>", "\n")
	text = strings.ReplaceAll(text, "</p>", "\n")
	text = tagRE.ReplaceAllString(text, " ")
	text = html.UnescapeString(text)
	text = wsRE.ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}

func docxXMLToText(data []byte) string {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var out strings.Builder
	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "p" {
				out.WriteByte('\n')
			}
		case xml.CharData:
			out.Write([]byte(t))
		}
	}
	return strings.TrimSpace(out.String())
}
