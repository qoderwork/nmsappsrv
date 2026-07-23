package pmfile

import "github.com/gin-gonic/gin"

// RegisterRoutes registers the web UI PM file management routes on the given
// router group. These are mounted under the /api/v2/ group in cmd/main.go and
// provide list/get/download/delete operations for PM files from the frontend.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	rg.POST("/pm-files/upload", h.Upload)
	rg.GET("/pm-files", h.ListFiles)
	rg.GET("/pm-files/:id", h.GetFile)
	rg.GET("/pm-files/:id/download", h.DownloadFile)
	rg.DELETE("/pm-files/:id", h.DeleteFile)
}

// RegisterPublicRoutes registers device-facing PM file upload routes that do
// not require web UI authentication. These mirror the Java PmController
// (base path @RequestMapping("/acs-file-server/")).
//
// NOTE: The Java routes accept the file as the raw request body with the file
// name in the path ({fileName}). The existing Go Upload handler expects a
// multipart form with a "file" field and "elementId" query parameter, so it
// does not match the Java contract. These routes are commented out as TODO
// until a raw-body upload handler (e.g. UploadNamed / UploadNoName) is added
// to handler.go, following the same pattern as filebase.UploadMrNamed.
func RegisterPublicRoutes(rg gin.IRouter, h *Handler) {
	// TODO: Java: PmController @PutMapping("/acs-file-server/pm/{fileName}")
	// rg.PUT("/acs-file-server/pm/:fileName", h.UploadNamed)

	// TODO: Java: PmController @PostMapping("/acs-file-server/pm/{fileName}")
	// rg.POST("/acs-file-server/pm/:fileName", h.UploadNamed)

	// TODO: Java: PmController @PostMapping("/acs-file-server/pm")
	//   and @PostMapping("/acs-file-server/pm/")
	//   Gin's RedirectTrailingSlash handles the trailing-slash variant.
	// rg.POST("/acs-file-server/pm", h.UploadNoName)

	// TODO: Java: PmController @PutMapping("/acs-file-server/pm")
	//   and @PutMapping("/acs-file-server/pm/")
	//   Gin's RedirectTrailingSlash handles the trailing-slash variant.
	// rg.PUT("/acs-file-server/pm", h.UploadNoName)
}
