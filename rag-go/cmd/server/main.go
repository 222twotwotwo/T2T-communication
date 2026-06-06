package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"llmentor/rag-go/internal/config"
	"llmentor/rag-go/internal/httpapi"
)

func main() {
	defaultConfig := os.Getenv("RAG_GO_CONFIG")
	if defaultConfig == "" {
		defaultConfig = "config.yaml"
		if _, err := os.Stat(defaultConfig); err != nil {
			defaultConfig = filepath.Join("rag-go", "config.yaml")
		}
	}

	configPath := flag.String("config", defaultConfig, "path to config.yaml")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	app, err := httpapi.New(cfg)
	if err != nil {
		log.Fatalf("create app: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := app.Init(ctx); err != nil {
		log.Printf("startup init warning: %v", err)
	}

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	log.Printf("rag-go listening on %s", addr)
	if err := http.ListenAndServe(addr, app.Routes()); err != nil {
		log.Fatal(err)
	}
}
