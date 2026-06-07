package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"t2t/backend/internal/config"
	"t2t/backend/internal/rag"
	"t2t/backend/internal/scenarios"
)

const maxRAGUploadBytes = 4 * 1024 * 1024

func ragIngestHandler(cfg config.AppConfig) gin.HandlerFunc {
	client := rag.NewClient(cfg.RAG)

	return func(c *gin.Context) {
		category := strings.TrimSpace(c.PostForm("category"))
		if category == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "category is required"})
			return
		}
		if _, err := scenarios.Get(category); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		file, err := c.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "markdown file is required"})
			return
		}
		if file.Size <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "markdown file is empty"})
			return
		}
		if file.Size > maxRAGUploadBytes {
			c.JSON(http.StatusBadRequest, gin.H{"error": "markdown file must be 4MB or smaller"})
			return
		}
		ext := strings.ToLower(filepath.Ext(file.Filename))
		if ext != ".md" && ext != ".markdown" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "only .md or .markdown files are supported"})
			return
		}

		tempDir, err := os.MkdirTemp("", "t2t-rag-upload-*")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer os.RemoveAll(tempDir)

		tempPath := filepath.Join(tempDir, safeUploadName(file.Filename))
		if err := c.SaveUploadedFile(file, tempPath); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		result, err := client.Ingest(c.Request.Context(), rag.IngestRequest{
			FilePath:        tempPath,
			Category:        category,
			DashScopeAPIKey: credentialsFromHeaders(c).DashScopeAPIKey,
		})
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status":      result.Status,
			"category":    category,
			"fileName":    file.Filename,
			"bytes":       file.Size,
			"ragResponse": result.RAGResponse,
		})
	}
}

func safeUploadName(name string) string {
	base := filepath.Base(strings.TrimSpace(name))
	if base == "" || base == "." || base == string(filepath.Separator) {
		return "knowledge.md"
	}
	return base
}
