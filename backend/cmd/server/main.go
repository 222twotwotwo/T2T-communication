package main

import (
	"log"

	"t2t/backend/internal/api"
	"t2t/backend/internal/config"
	"t2t/backend/internal/providers"
	"t2t/backend/internal/session"
)

func main() {
	cfg, warnings := config.Load()
	for _, warning := range warnings {
		log.Printf("config warning: %s", warning)
	}

	bundle := providers.NewProviderBundle(cfg)
	service := session.NewService(bundle)
	router := api.NewRouter(cfg, service)

	addr := ":" + cfg.Server.Port
	log.Printf("T2T backend listening on %s with provider mode=%s", addr, cfg.Providers.Mode)
	if err := router.Run(addr); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
}
