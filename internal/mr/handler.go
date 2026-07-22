package mr

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"nmsappsrv/pkg/utils"
)

// Handler exposes the MR admin/report endpoints (Java MRManagementController),
// mounted under the authenticated /api/v1 group.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func tenantID(c *gin.Context) int {
	if v, ok := c.Get("tenant_id"); ok {
		switch t := v.(type) {
		case int:
			return t
		case int64:
			return int(t)
		case float64:
			return int(t)
		}
	}
	return 0
}

func parseTimeFlex(s string) (time.Time, error) {
	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05.000",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid time: %s", s)
}

type heatmapReq struct {
	MRName    string `json:"mrName"`
	ElementID int64  `json:"elementId"`
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
}

// GetStatisticData handles POST /getMRStatisticDataForData (heatmap).
func (h *Handler) GetStatisticData(c *gin.Context) {
	var req heatmapReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request")
		return
	}
	start, err1 := parseTimeFlex(req.StartTime)
	end, err2 := parseTimeFlex(req.EndTime)
	if err1 != nil || err2 != nil || req.MRName == "" || req.ElementID == 0 {
		utils.Error(c, http.StatusBadRequest, "mrName, elementId, startTime and endTime are required")
		return
	}
	vo, err := h.svc.GetHeatmap(HeatmapQuery{
		MRName:    req.MRName,
		ElementID: req.ElementID,
		StartTime: start,
		EndTime:   end,
	})
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, vo)
}

type csvReq struct {
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
}

// GenerateCSV handles POST /generateMRCSVForNR. tenantId comes from the auth
// context (Java's SecurityUtil.getTenantId()).
func (h *Handler) GenerateCSV(c *gin.Context) {
	var req csvReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request")
		return
	}
	start, err1 := parseTimeFlex(req.StartTime)
	end, err2 := parseTimeFlex(req.EndTime)
	if err1 != nil || err2 != nil {
		utils.Error(c, http.StatusBadRequest, "startTime and endTime are required")
		return
	}
	fileName, err := h.svc.GenerateCSV(tenantID(c), start, end)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Success(c, fileName)
}

// DownloadReport handles GET /downloadMRReportExcel?fileName= (Java serves and
// deletes the xlsx; we keep it).
func (h *Handler) DownloadReport(c *gin.Context) {
	fileName := c.Query("fileName")
	path, err := h.svc.ReportFilePath(fileName)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	c.File(path)
}

// ListLogs handles POST /listMRLogs.
func (h *Handler) ListLogs(c *gin.Context) {
	var req struct {
		Page     int `json:"page"`
		PageSize int `json:"pageSize"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		req.Page = 1
		req.PageSize = 20
	}
	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 {
		req.PageSize = 20
	}
	logs, total, err := h.svc.ListLogs(req.Page, req.PageSize)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	utils.Paginated(c, logs, total, req.Page, req.PageSize)
}

// DownloadRaw handles GET /downloadMRFile?id= (serves the raw uploaded file).
func (h *Handler) DownloadRaw(c *gin.Context) {
	id, err := strconv.ParseInt(c.Query("id"), 10, 64)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid id")
		return
	}
	path, err := h.svc.RawFilePath(id)
	if err != nil {
		utils.Error(c, http.StatusNotFound, "file not found")
		return
	}
	c.File(path)
}
