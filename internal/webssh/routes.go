package webssh

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes registers the WebSSH WebSocket route on the given router group.
//
// Endpoint: GET /webssh
// The client must send a JSON "connect" message with host/port/username/password
// immediately after the WebSocket is established.
func RegisterRoutes(rg *gin.RouterGroup) {
	svc := NewService()

	// GET /webssh -- WebSocket endpoint for interactive SSH terminal
	rg.GET("/webssh", func(c *gin.Context) {
		HandleWebSSH(c, svc)
	})
}
