package webssh

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// RegisterRoutes registers the WebSSH WebSocket route on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, db *gorm.DB) {
	svc := NewService(db)

	// GET /webssh/:deviceId -- WebSocket endpoint for interactive SSH terminal
	rg.GET("/webssh/:deviceId", func(c *gin.Context) {
		HandleWebSSH(c, svc)
	})
}
