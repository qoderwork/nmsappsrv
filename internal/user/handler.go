package user

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"nmsappsrv/internal/authz"
	"nmsappsrv/internal/captcha"
	"nmsappsrv/internal/middleware"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/utils"

	"gorm.io/gorm"
)

// OnlineChecker checks whether a user is currently online via WebSocket
// heartbeat. Implemented by websocket.Hub to avoid a circular dependency.
type OnlineChecker interface {
	IsUserOnline(username string) bool
}

// Handler exposes HTTP handlers for user-related endpoints.
type Handler struct {
	svc            Service
	db             *gorm.DB
	captchaMgr     *captcha.Manager
	onlineChecker  OnlineChecker
}

// NewHandler creates a Handler backed by a fresh Service. captchaMgr may be nil
// (e.g. when captcha is not configured); in that case the login captcha gate is
// skipped entirely. onlineChecker may be nil (e.g. in tests); in that case
// LoginState is always false.
func NewHandler(db *gorm.DB, captchaMgr *captcha.Manager, onlineChecker OnlineChecker) *Handler {
	return &Handler{svc: NewService(db), db: db, captchaMgr: captchaMgr, onlineChecker: onlineChecker}
}

// ---------------------------------------------------------------------------
// Auth endpoints
// ---------------------------------------------------------------------------

// loginRequest is the expected JSON body for the Login endpoint.
type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	// VerificationKey/VerificationCode are only required when the adaptive
	// captcha gate has been triggered for this username+IP (see captcha.Manager).
	VerificationKey  string `json:"verificationKey"`
	VerificationCode string `json:"verificationCode"`
}

// Login handles POST /login
func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	ip := c.ClientIP()

	// Adaptive captcha gate: only enforced once failures have triggered it.
	if h.captchaMgr != nil && h.captchaMgr.IsRequired(req.Username, ip) {
		if req.VerificationKey == "" || req.VerificationCode == "" {
			utils.ErrorWithExtra(c, http.StatusBadRequest, "captcha required", map[string]interface{}{"required": true})
			return
		}
		if !h.captchaMgr.Verify(c.Request.Context(), req.VerificationKey, req.VerificationCode) {
			h.captchaMgr.OnFailure(req.Username, ip)
			utils.ErrorWithExtra(c, http.StatusBadRequest, "invalid captcha", map[string]interface{}{"required": true})
			return
		}
	}

	u, err := h.svc.Login(req.Username, req.Password)
	if err != nil {
		// Record failed login attempt + bump the captcha risk counters.
		if h.captchaMgr != nil {
			h.captchaMgr.OnFailure(req.Username, ip)
		}
		_ = h.svc.RecordLogin(req.Username, c.ClientIP(), 0, 0)
		utils.HandleError(c, err)
		return
	}

	// Successful login resets the captcha risk state.
	if h.captchaMgr != nil {
		h.captchaMgr.OnSuccess(req.Username, ip)
	}

	tenantId := 0 // 0 = platform user (Admin/Operator), no tenant filter (aligns with Java SecurityUtil.getTenantId() returning null)
	if u.TenantId != nil && *u.TenantId > 0 {
		tenantId = *u.TenantId
	}

	// Resolve role names for JWT claims (aligned with Java JWT structure)
	roleNames, _ := h.svc.GetRoleNamesForUser(u.Id, tenantId)

	token, err := middleware.GenerateToken(u.Id, *u.Username, tenantId, roleNames, "")
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "failed to generate token")
		return
	}

	// Record successful login.
	_ = h.svc.RecordLogin(req.Username, c.ClientIP(), tenantId, 1)

	utils.Success(c, gin.H{"token": token})
}

// CaptchaImage handles GET /captchaImage: issues a new digit captcha and
// returns its key + base64 image so the frontend can render and submit it.
func (h *Handler) CaptchaImage(c *gin.Context) {
	if h.captchaMgr == nil {
		utils.Error(c, http.StatusServiceUnavailable, "captcha not configured")
		return
	}
	key, b64, err := h.captchaMgr.Generate(c.Request.Context())
	if err != nil {
		logger.Errorf("captcha generate failed: %v", err)
		utils.Error(c, http.StatusInternalServerError, "failed to generate captcha")
		return
	}
	utils.Success(c, gin.H{"key": key, "imageBase64": b64})
}

// Logout handles POST /logout
func (h *Handler) Logout(c *gin.Context) {
	username := middleware.GetUsername(c)
	tenantId := middleware.GetTenantId(c)

	// Extract JWT from Authorization header
	authHeader := c.GetHeader("Authorization")
	jwtToken := ""
	if parts := strings.SplitN(authHeader, " ", 2); len(parts) == 2 {
		jwtToken = strings.TrimSpace(parts[1])
	}

	if jwtToken != "" && username != "" {
		_ = h.svc.Logout(username, jwtToken)
	}

	// Record logout
	_ = h.svc.RecordLogout(username, c.ClientIP(), tenantId)

	utils.Success(c, nil)
}

// ---------------------------------------------------------------------------
// User endpoints
// ---------------------------------------------------------------------------

// ListUsers handles GET /users?page=1&pageSize=20
// Mirrors Java SystemUserManagementServiceImpl.listUser:
//   - excludes admin user
//   - non-admin users can only see users they created
//   - returns fields aligned with Java ListUserVO
func (h *Handler) ListUsers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	username := middleware.GetUsername(c)
	roleNames := middleware.GetRoleNames(c)

	// Determine if current user is admin (mirrors Java authorityHelper.isAdminRole)
	isAdmin := false
	for _, r := range roleNames {
		if strings.EqualFold(r, "admin") {
			isAdmin = true
			break
		}
	}

	// Non-admin users can only see users they created
	creatorName := ""
	if !isAdmin {
		creatorName = username
	}

	data, total, err := h.svc.ListUsers(page, pageSize, true, creatorName)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	// Convert to DTOs to exclude sensitive fields (password, salt)
	dtos := ToUserDTOs(data)

	// Fill tenancy names (mirrors Java: tenantIdToNameMap)
	// Build lookup from tenant_id -> license_name
	tenantIds := make(map[int]bool)
	for _, u := range data {
		if u.TenantId != nil {
			tenantIds[*u.TenantId] = true
		}
	}
	var tenancyMap map[int]string
	if len(tenantIds) > 0 {
		tenancyMap = h.buildTenancyMap(tenantIds)
	}

	for i := range dtos {
		// Fill tenancy name
		if data[i].TenantId != nil && tenancyMap != nil {
			dtos[i].Tenancy = tenancyMap[*data[i].TenantId]
		}
		// Default createUsername to "admin" if empty (mirrors Java)
		if dtos[i].CreateUsername == nil || *dtos[i].CreateUsername == "" {
			admin := "admin"
			dtos[i].CreateUsername = &admin
		}
	}

	// Fill LoginState from WebSocket heartbeat (mirrors Java's
	// lastHeartbeatTime check in SystemUserManagementServiceImpl).
	if h.onlineChecker != nil {
		for i := range dtos {
			if dtos[i].Username != nil {
				dtos[i].LoginState = h.onlineChecker.IsUserOnline(*dtos[i].Username)
			}
		}
	}
	utils.Paginated(c, dtos, total, page, pageSize)
}

// buildTenancyMap queries the license table and returns a map of id -> license_name.
func (h *Handler) buildTenancyMap(tenantIds map[int]bool) map[int]string {
	result := make(map[int]string)
	var ids []int
	for id := range tenantIds {
		ids = append(ids, id)
	}
	type licenseRow struct {
		Id          int
		LicenseName *string `gorm:"column:license_name"`
	}
	var rows []licenseRow
	if err := h.db.Table("tenant").Select("id, license_name").Where("id IN ?", ids).Find(&rows).Error; err != nil {
		logger.Warnf("buildTenancyMap: failed to query licenses: %v", err)
		return result
	}
	for _, r := range rows {
		if r.LicenseName != nil {
			result[r.Id] = *r.LicenseName
		}
	}
	return result
}

// CreateUser handles POST /users
// Returns AddUserVO with userId and generated password.
func (h *Handler) CreateUser(c *gin.Context) {
	var u SysUser
	if err := c.ShouldBindJSON(&u); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	// Set creator and license (tenancy) — mirrors Java SecurityUtil.getTenantId()
	// Admin/Operator roles are not restricted by tenancy isolation (returns null in Java)
	creatorId := middleware.GetUserId(c)
	u.CreateUserId = &creatorId
	roleNames := middleware.GetRoleNames(c)
	isAdminOrOperator := false
	for _, r := range roleNames {
		if strings.EqualFold(r, "admin") || strings.EqualFold(r, "operator") {
			isAdminOrOperator = true
			break
		}
	}
	if !isAdminOrOperator {
		tenantId := middleware.GetTenantId(c)
		if tenantId > 0 {
			u.TenantId = &tenantId
		}
	}

	password, err := h.svc.CreateUser(&u)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	// Return AddUserVO for new users (mirrors Java behavior)
	utils.Success(c, &AddUserVO{
		UserId:   u.Id,
		Password: password,
	})
}

// UpdateUser handles PUT /users/:id
func (h *Handler) UpdateUser(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid user id")
		return
	}

	var u SysUser
	if err := c.ShouldBindJSON(&u); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	u.Id = id

	if err := h.svc.UpdateUser(&u); err != nil {
		utils.HandleError(c, err)
		return
	}
	// Return DTO to exclude sensitive fields (password, salt)
	dto := ToUserDTO(&u)
	utils.Success(c, &dto)
}

// DeleteUser handles DELETE /users/:id
func (h *Handler) DeleteUser(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid user id")
		return
	}

	if err := h.svc.DeleteUser(id); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, nil)
}

// ---------------------------------------------------------------------------
// User Management endpoints
// ---------------------------------------------------------------------------

// KickOutUser handles POST /users/kick-out
func (h *Handler) KickOutUser(c *gin.Context) {
	var req struct {
		UserId int `json:"userId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.svc.KickOutUser(req.UserId); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, nil)
}

// UnlockUser handles POST /users/unlock
func (h *Handler) UnlockUser(c *gin.Context) {
	var req struct {
		UserId int `json:"userId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.svc.UnlockUser(req.UserId); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, nil)
}

// ModifyPassword handles POST /users/modify-password
func (h *Handler) ModifyPassword(c *gin.Context) {
	var req ModifyPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	username := middleware.GetUsername(c)
	if err := h.svc.ModifyPassword(username, req.OldPassword, req.NewPassword); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, nil)
}

// EnableUser handles POST /users/enable
func (h *Handler) EnableUser(c *gin.Context) {
	var req struct {
		UserId int `json:"userId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.svc.EnableUser(req.UserId); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, nil)
}

// DisableUser handles POST /users/disable
func (h *Handler) DisableUser(c *gin.Context) {
	var req struct {
		UserId int `json:"userId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.svc.DisableUser(req.UserId); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, nil)
}

// ResetPassword handles POST /users/reset-password
// Returns the newly generated password.
func (h *Handler) ResetPassword(c *gin.Context) {
	var req ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	adminId := middleware.GetUserId(c)
	password, err := h.svc.ResetPassword(adminId, req.UserId)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, &AddUserVO{
		UserId:   req.UserId,
		Password: password,
	})
}

// ResetPasswordByLink handles POST /users/reset-password-by-link
func (h *Handler) ResetPasswordByLink(c *gin.Context) {
	var req ResetPasswordByLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.svc.ResetPasswordByLink(req.Username, req.Key, req.NewPassword); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, nil)
}

// SetTenancyForUser handles POST /users/set-tenancy
func (h *Handler) SetTenancyForUser(c *gin.Context) {
	var req SetTenancyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.svc.SetTenancyForUser(req.UserId, req.TenantId); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, nil)
}

// GetLoginFailedTimes handles POST /users/login-failed-times
func (h *Handler) GetLoginFailedTimes(c *gin.Context) {
	var req struct {
		UserId int `json:"userId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	data, err := h.svc.GetLoginFailedTimes(req.UserId)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, data)
}

// NeedChangePassword handles GET /users/need-change-password
func (h *Handler) NeedChangePassword(c *gin.Context) {
	userId := middleware.GetUserId(c)

	data, err := h.svc.NeedChangePassword(userId)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, data)
}

// ---------------------------------------------------------------------------
// Role endpoints
// ---------------------------------------------------------------------------

// ListRoles handles GET /roles
func (h *Handler) ListRoles(c *gin.Context) {
	tenantId := middleware.GetTenantId(c)

	data, err := h.svc.ListRoles(tenantId)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, data)
}

// CreateRole handles POST /roles
func (h *Handler) CreateRole(c *gin.Context) {
	var r Role
	if err := c.ShouldBindJSON(&r); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.svc.CreateRole(&r); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, &r)
}

// UpdateRole handles PUT /roles/:id
func (h *Handler) UpdateRole(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		utils.Error(c, http.StatusBadRequest, "invalid role id")
		return
	}

	var r Role
	if err := c.ShouldBindJSON(&r); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	r.Id = id

	if err := h.svc.UpdateRole(&r); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, &r)
}

// DeleteRole handles DELETE /roles/:id
func (h *Handler) DeleteRole(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		utils.Error(c, http.StatusBadRequest, "invalid role id")
		return
	}

	if err := h.svc.DeleteRole(id); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, nil)
}

// ---------------------------------------------------------------------------
// Role Permission endpoints
// ---------------------------------------------------------------------------

// permissionRequest is the expected JSON body for updating role permissions.
type permissionRequest struct {
	PermissionIds []string `json:"permission_ids"`
}

// GetRolePermissions handles GET /roles/:id/permissions
func (h *Handler) GetRolePermissions(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		utils.Error(c, http.StatusBadRequest, "invalid role id")
		return
	}

	data, err := h.svc.GetRolePermissions(id)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, data)
}

// UpdateRolePermissions handles PUT /roles/:id/permissions
func (h *Handler) UpdateRolePermissions(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		utils.Error(c, http.StatusBadRequest, "invalid role id")
		return
	}

	var req permissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.svc.UpdateRolePermissions(id, req.PermissionIds); err != nil {
		utils.HandleError(c, err)
		return
	}
	// Keep the in-memory casbin policy in sync with role_has_permission.
	if err := authz.Reload(h.db); err != nil {
		logger.Errorf("failed to reload RBAC policy after role permission update: %v", err)
	}
	utils.Success(c, nil)
}
