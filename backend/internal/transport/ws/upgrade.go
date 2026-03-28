package ws

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/kanielv/mafiacv/backend/internal/models"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins in development
	},
}

func ServeWS(hub *Hub) gin.HandlerFunc {
	return func(c *gin.Context) {
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Printf("websocket upgrade error: %v", err)
			return
		}

		client := &Client{
			ID:   generateClientID(),
			Conn: conn,
			Hub:  hub,
			Send: make(chan []byte, 256),
		}

		hub.Register <- client

		// Send the client their ID so the frontend can track it
		idMsg := MarshalMessage("connected", map[string]string{"socketId": client.ID})
		client.Send <- idMsg

		go client.WritePump()
		go client.ReadPump()
	}
}

func generateClientID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// MarshalMessage creates a JSON-encoded WSMessage.
func MarshalMessage(event string, data any) []byte {
	dataBytes, _ := json.Marshal(data)
	msg := models.WSMessage{
		Event: event,
		Data:  dataBytes,
	}
	out, _ := json.Marshal(msg)
	return out
}
