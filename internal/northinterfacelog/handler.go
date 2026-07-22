package northinterfacelog

import (
	"time"

	"github.com/gin-gonic/gin"

	"nmsappsrv/internal/middleware"
	"nmsappsrv/pkg/utils"
)

// listRequest accepts both a flat shape (searchText/startTime/endTime/
// pageNumber/pageSize) and the Java RequestDataDTO wrapper
// ({query:{...}, page:{pageNumber,pageSize}}) for drop-in compatibility with
// clients written against the Java endpoint.
type listRequest struct {
	SearchText string `json:"searchText"`
	StartTime  string `json:"startTime"`
	EndTime    string `json:"endTime"`
	PageNumber int    `json:"pageNumber"`
	PageSize   int    `json:"pageSize"`
	Query      *struct {
		SearchText string `json:"searchText"`
		StartTime  string `json:"startTime"`
		EndTime    string `json:"endTime"`
	} `json:"query"`
	PageInfo *struct {
		PageNumber int `json:"pageNumber"`
		PageSize   int `json:"pageSize"`
	} `json:"page"`
}

// Handler exposes the northbound audit-log query endpoint.
type Handler struct {
	svc *Service
}

// NewHandler builds a Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// ListLogs handles POST /api/v1/north-interface-logs. It
// filters by the caller's tenancy, an optional searchText (log_name or the
// device referenced by element_id), and an optional time window, returning a
// 1-based page ordered by operationTime descending.
func (h *Handler) ListLogs(c *gin.Context) {
	var req listRequest
	_ = c.ShouldBindJSON(&req)

	searchText := req.SearchText
	startTime := req.StartTime
	endTime := req.EndTime
	page := req.PageNumber
	pageSize := req.PageSize
	if req.Query != nil {
		if req.Query.SearchText != "" {
			searchText = req.Query.SearchText
		}
		if req.Query.StartTime != "" {
			startTime = req.Query.StartTime
		}
		if req.Query.EndTime != "" {
			endTime = req.Query.EndTime
		}
	}
	if req.PageInfo != nil {
		if req.PageInfo.PageNumber != 0 {
			page = req.PageInfo.PageNumber
		}
		if req.PageInfo.PageSize != 0 {
			pageSize = req.PageInfo.PageSize
		}
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	f := ListFilter{TenancyID: middleware.GetTenantId(c)}
	if searchText != "" {
		f.SearchText = searchText
	}
	if t, err := parseTimeFlex(startTime); err == nil {
		f.StartTime = &t
	}
	if t, err := parseTimeFlex(endTime); err == nil {
		f.EndTime = &t
	}

	vos, total, err := h.svc.List(f, page, pageSize)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Paginated(c, vos, total, page, pageSize)
}

// parseTimeFlex parses several common timestamp layouts.
func parseTimeFlex(s string) (time.Time, error) {
	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	var lastErr error
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t, nil
		} else {
			lastErr = err
		}
	}
	return time.Time{}, lastErr
}
