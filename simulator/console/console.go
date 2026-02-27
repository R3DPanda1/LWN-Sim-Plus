package console

import (
	"log"

	socketio "github.com/googollee/go-socket.io"
)

type Console struct {
	WebSocket *socketio.Conn // Pointer so all device/gateway copies share the same connection
	WatchedID *int           // Pointer so all device copies share the same value
}

func (c *Console) IsWatched(deviceID int) bool {
	return c.WatchedID != nil && *c.WatchedID == deviceID
}

func (c *Console) PrintLog(message string) {
	log.Println(message)
}

func (c *Console) PrintSocket(eventName string, data ...interface{}) {
	if c.WebSocket != nil && *c.WebSocket != nil {
		(*c.WebSocket).Emit(eventName, data...)
	}
}

func (c *Console) SetupWebSocket(WebSocket *socketio.Conn) {
	*c.WebSocket = *WebSocket
}
