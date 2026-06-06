package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"t2t/backend/internal/config"
	"t2t/backend/internal/domain"
	"t2t/backend/internal/scenarios"
	"t2t/backend/internal/session"
)

func NewRouter(cfg config.AppConfig, service *session.Service) *gin.Engine {
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery(), cors(cfg.Server.CorsOrigins))

	router.GET("/api/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "t2t-backend"})
	})

	router.GET("/api/scenarios", func(c *gin.Context) {
		c.JSON(http.StatusOK, scenarios.List())
	})

	router.GET("/api/provider-status", func(c *gin.Context) {
		c.JSON(http.StatusOK, service.ProviderStatus())
	})

	router.POST("/api/sessions", func(c *gin.Context) {
		var request session.CreateRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		request.Credentials = credentialsFromHeaders(c)
		created, err := service.Create(c.Request.Context(), request)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, created)
	})

	router.GET("/api/sessions/:id", func(c *gin.Context) {
		found, err := service.Get(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, found)
	})

	router.POST("/api/sessions/:id/turn", func(c *gin.Context) {
		var request session.TurnRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		request.Credentials = credentialsFromHeaders(c)
		response, err := service.AddTurn(c.Request.Context(), c.Param("id"), request)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, response)
	})

	router.POST("/api/sessions/:id/finish", func(c *gin.Context) {
		report, err := service.Finish(c.Request.Context(), c.Param("id"), credentialsFromHeaders(c))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, report)
	})

	router.GET("/api/realtime/:id", realtimeHandler(service))

	return router
}

func credentialsFromHeaders(c *gin.Context) domain.RuntimeCredentials {
	return domain.RuntimeCredentials{
		LLMProvider:     strings.ToLower(strings.TrimSpace(c.GetHeader("X-T2T-LLM-Provider"))),
		OpenAIAPIKey:    strings.TrimSpace(c.GetHeader("X-T2T-OpenAI-Key")),
		AnthropicAPIKey: strings.TrimSpace(c.GetHeader("X-T2T-Anthropic-Key")),
		DashScopeAPIKey: strings.TrimSpace(c.GetHeader("X-T2T-DashScope-Key")),
	}
}

func cors(origins []string) gin.HandlerFunc {
	allowed := map[string]bool{}
	for _, origin := range origins {
		allowed[strings.TrimSpace(origin)] = true
	}
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		if allowed[origin] || len(allowed) == 0 {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
		}
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-T2T-LLM-Provider, X-T2T-OpenAI-Key, X-T2T-Anthropic-Key, X-T2T-DashScope-Key")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
