package parameter

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"nmsappsrv/pkg/utils"
)

// ---------------------------------------------------------------------------
// TR-069 Parameter Definition – Service implementation
// ---------------------------------------------------------------------------

// AddTR069Parameter persists a new TR-069 parameter definition.
func (s *service) AddTR069Parameter(param *TR069Parameter) error {
	param.CreateTime = time.Now()
	return s.repo.CreateTR069Parameter(param)
}

// ListTR069Parameters returns a paginated list of TR-069 parameter definitions.
func (s *service) ListTR069Parameters(page, pageSize int) ([]TR069Parameter, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.repo.FindTR069Parameters(offset, pageSize)
}

// ViewTR069Parameter returns a single TR-069 parameter definition by ID.
func (s *service) ViewTR069Parameter(id int64) (*TR069Parameter, error) {
	return s.repo.FindTR069ParameterByID(id)
}

// UpdateTR069Parameter persists changes to an existing TR-069 parameter definition.
func (s *service) UpdateTR069Parameter(param *TR069Parameter) error {
	return s.repo.UpdateTR069Parameter(param)
}

// DeleteTR069Parameter removes a TR-069 parameter definition by ID.
func (s *service) DeleteTR069Parameter(id int64) error {
	return s.repo.DeleteTR069Parameter(id)
}

// ---------------------------------------------------------------------------
// TR-069 Parameter Definition – HTTP handlers
// ---------------------------------------------------------------------------

// AddTR069Parameter handles POST /tr069-parameters
func (h *Handler) AddTR069Parameter(c *gin.Context) {
	var param TR069Parameter
	if err := c.ShouldBindJSON(&param); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	if param.ParameterName == "" {
		utils.Error(c, http.StatusBadRequest, "parameter_name is required")
		return
	}

	if err := h.svc.AddTR069Parameter(&param); err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to add TR069 parameter")
		return
	}
	utils.Success(c, param)
}

// ListTR069Parameters handles GET /tr069-parameters?page=&pageSize=
func (h *Handler) ListTR069Parameters(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	data, total, err := h.svc.ListTR069Parameters(page, pageSize)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to list TR069 parameters")
		return
	}
	utils.Paginated(c, data, total, page, pageSize)
}

// ViewTR069Parameter handles GET /tr069-parameters/:id
func (h *Handler) ViewTR069Parameter(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid parameter id")
		return
	}

	param, err := h.svc.ViewTR069Parameter(id)
	if err != nil {
		utils.Error(c, http.StatusNotFound, "TR069 parameter not found")
		return
	}
	utils.Success(c, param)
}

// UpdateTR069Parameter handles PUT /tr069-parameters/:id
func (h *Handler) UpdateTR069Parameter(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid parameter id")
		return
	}

	var param TR069Parameter
	if err := c.ShouldBindJSON(&param); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	param.Id = id

	if err := h.svc.UpdateTR069Parameter(&param); err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to update TR069 parameter")
		return
	}
	utils.Success(c, param)
}

// DeleteTR069Parameter handles DELETE /tr069-parameters/:id
func (h *Handler) DeleteTR069Parameter(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid parameter id")
		return
	}

	if err := h.svc.DeleteTR069Parameter(id); err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to delete TR069 parameter")
		return
	}
	utils.Success(c, nil)
}
