package user

import (
	"github.com/gin-gonic/gin"

	"nmsappsrv/internal/authz"
)

// RegisterRoutes registers all user management routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// 用户管理
	rg.GET("/users", authz.RequirePermission("System.Authority.User.ListUser"), h.ListUsers)
	rg.POST("/users", authz.RequirePermission("System.Authority.User.AddUser"), h.CreateUser)
	rg.PUT("/users/:id", authz.RequirePermission("System.Authority.User.ModifyUser"), h.UpdateUser)
	rg.DELETE("/users/:id", authz.RequirePermission("System.Authority.User.DeleteUser"), h.DeleteUser)
	rg.POST("/users/kick-out", authz.RequirePermission("System.Authority.User.KickOutUser"), h.KickOutUser)
	rg.POST("/users/unlock", authz.RequirePermission("System.Authority.User.EnableUser"), h.UnlockUser)
	rg.GET("/renewToken", h.RenewToken)
	rg.POST("/users/modify-password", h.ModifyPassword)
	rg.POST("/users/enable", authz.RequirePermission("System.Authority.User.EnableUser"), h.EnableUser)
	rg.POST("/users/disable", authz.RequirePermission("System.Authority.User.DisableUser"), h.DisableUser)
	rg.POST("/users/reset-password", authz.RequirePermission("System.Authority.User.ResetPassword"), h.ResetPassword)
	rg.POST("/users/reset-password-by-link", h.ResetPasswordByLink)
	rg.POST("/users/set-tenancy", h.SetTenancyForUser)
	rg.POST("/users/login-failed-times", h.GetLoginFailedTimes)
	rg.GET("/users/need-change-password", h.NeedChangePassword)
	rg.GET("/roles", authz.RequirePermission("System.Authority.Role.ListRole"), h.ListRoles)
	rg.POST("/roles", authz.RequirePermission("System.Authority.Role.AddRole"), h.CreateRole)
	rg.PUT("/roles/:id", authz.RequirePermission("System.Authority.Role.UpdateRole"), h.UpdateRole)
	rg.DELETE("/roles/:id", authz.RequirePermission("System.Authority.Role.DeleteRole"), h.DeleteRole)
	rg.GET("/roles/:id/permissions", authz.RequirePermission("System.Authority.Role.GetRolePermissions"), h.GetRolePermissions)
	rg.PUT("/roles/:id/permissions", authz.RequirePermission("System.Authority.Role.UpdateRolePermissions"), h.UpdateRolePermissions)

	// Login logs
	rg.GET("/users/loginlog", authz.RequirePermission("System.Log.ListLoginLog"), h.ListLoginLogs)
}

// RegisterPublicRoutes registers public routes (no auth required) on the given router group.
func RegisterPublicRoutes(rg *gin.RouterGroup, h *Handler) {
	rg.POST("/login", h.Login)
	rg.POST("/logout", h.Logout)
	rg.GET("/captchaImage", h.CaptchaImage)
}
