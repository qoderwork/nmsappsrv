package user

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all user management routes on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// 用户管理
	rg.GET("/users", h.ListUsers)
	rg.POST("/users", h.CreateUser)
	rg.PUT("/users/:id", h.UpdateUser)
	rg.DELETE("/users/:id", h.DeleteUser)
	rg.POST("/users/kick-out", h.KickOutUser)
	rg.POST("/users/unlock", h.UnlockUser)
	rg.POST("/users/modify-password", h.ModifyPassword)
	rg.POST("/users/enable", h.EnableUser)
	rg.POST("/users/disable", h.DisableUser)
	rg.POST("/users/reset-password", h.ResetPassword)
	rg.POST("/users/reset-password-by-link", h.ResetPasswordByLink)
	rg.POST("/users/set-tenancy", h.SetTenancyForUser)
	rg.POST("/users/login-failed-times", h.GetLoginFailedTimes)
	rg.GET("/users/need-change-password", h.NeedChangePassword)
	rg.GET("/roles", h.ListRoles)
	rg.POST("/roles", h.CreateRole)
	rg.PUT("/roles/:id", h.UpdateRole)
	rg.DELETE("/roles/:id", h.DeleteRole)
	rg.GET("/roles/:id/permissions", h.GetRolePermissions)
	rg.PUT("/roles/:id/permissions", h.UpdateRolePermissions)
}

// RegisterPublicRoutes registers public routes (no auth required) on the given router group.
func RegisterPublicRoutes(rg *gin.RouterGroup, h *Handler) {
	rg.POST("/login", h.Login)
	rg.POST("/logout", h.Logout)
}
