package diagnostics

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"nmsappsrv/pkg/utils"
)

// ---------- Diagnostics task history ----------

// DiagnosticsTaskVO is the exposed diagnostics task view model.
type DiagnosticsTaskVO struct {
	Id         int64  `json:"id"`
	ElementId  int64  `json:"elementId"`
	TaskType   string `json:"taskType"`
	Status     *int   `json:"status"`
	Result     string `json:"result"`
	Command    string `json:"command"`
	CreateTime string `json:"createTime"`
	EndTime    string `json:"endTime"`
}

// ListDiagnosticsTasks handles GET /diagnostics/tasks?page=&pageSize=
func (h *Handler) ListDiagnosticsTasks(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	utils.Paginated(c, []DiagnosticsTaskVO{}, int64(0), page, pageSize)
}

// GetDiagnosticsTask handles GET /diagnostics/tasks/:id
func (h *Handler) GetDiagnosticsTask(c *gin.Context) {
	utils.Error(c, http.StatusNotFound, "diagnostics task not found")
}
