package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"t2t/backend/internal/session"
)

type realtimeInbound struct {
	Type        string `json:"type"`
	Text        string `json:"text"`
	AudioBase64 string `json:"audioBase64"`
	MimeType    string `json:"mimeType"`
	Final       bool   `json:"final"`
}

type realtimeOutbound struct {
	Type    string `json:"type"`
	Payload any    `json:"payload"`
	Error   string `json:"error,omitempty"`
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(_ *http.Request) bool {
		return true
	},
}

func realtimeHandler(service *session.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		sessionID := c.Param("id")
		bufferedText := strings.Builder{}
		bufferedAudio := strings.Builder{}
		mimeType := ""

		_ = conn.WriteJSON(realtimeOutbound{Type: "ready", Payload: map[string]string{"sessionId": sessionID}})

		for {
			var inbound realtimeInbound
			if err := conn.ReadJSON(&inbound); err != nil {
				return
			}
			switch inbound.Type {
			case "user_text":
				if inbound.Text != "" {
					bufferedText.WriteString(" ")
					bufferedText.WriteString(inbound.Text)
				}
			case "audio_chunk":
				if inbound.AudioBase64 != "" {
					bufferedAudio.WriteString(inbound.AudioBase64)
				}
				if inbound.MimeType != "" {
					mimeType = inbound.MimeType
				}
			case "finish_turn":
				inbound.Final = true
			}

			if !inbound.Final {
				continue
			}

			request := session.TurnRequest{
				Text:        strings.TrimSpace(bufferedText.String()),
				AudioBase64: bufferedAudio.String(),
				MimeType:    mimeType,
			}
			bufferedText.Reset()
			bufferedAudio.Reset()
			mimeType = ""

			turn, err := service.AddTurn(c.Request.Context(), sessionID, request)
			if err != nil {
				_ = conn.WriteJSON(realtimeOutbound{Type: "error", Error: err.Error()})
				continue
			}
			_ = conn.WriteJSON(realtimeOutbound{Type: "transcript", Payload: turn.UserMessage})
			_ = conn.WriteJSON(realtimeOutbound{Type: "assistant_message", Payload: turn.AssistantMessage})
			_ = conn.WriteJSON(realtimeOutbound{Type: "turn_signal", Payload: turn.Signal})
		}
	}
}
