package initserver

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all init server routes on the given router group.
// Route paths mirror the Java InitServerController (no class-level base path,
// @Controller annotation). The group is mounted under /api/v2/ in cmd/main.go,
// so the full paths become /api/v2/getConfig, /api/v2/save, etc.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// Java: InitServerController @GetMapping("/getConfig")
	rg.GET("/getConfig", h.GetConfig)
	// Java: InitServerController @PostMapping("/save")
	rg.POST("/save", h.SaveConfig)
	// Java: InitServerController @GetMapping("/exportConfig")
	rg.GET("/exportConfig", h.ExportConfig)
	// Java: InitServerController @PostMapping("/uploadConfig")
	rg.POST("/uploadConfig", h.UploadConfig)

	// NOTE: Java InitServerController @PostMapping("/init-server") (dealInitRequest)
	// is a device-facing public route (no web UI auth). It is registered directly
	// on the root router in cmd/main.go:
	//   router.POST("/init-server", initserverH.HandleInitServer)
	// It is intentionally NOT registered here to avoid placing it behind the
	// authenticated /api/v2/ group.
}
