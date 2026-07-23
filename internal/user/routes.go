package user

import (
	"github.com/gin-gonic/gin"

	"nmsappsrv/internal/authz"
)

// RegisterRoutes registers all user management routes on the given router group.
// Aligned with Java SystemUserController and RoleManagementController:
// @RequestMapping("/api/v2/") + @PostMapping("methodName")
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	// 用户管理 - Java: SystemUserController
	rg.POST("/addUser", authz.RequirePermission("System.Authority.User.AddUser"), h.CreateUser)
	rg.POST("/listUser", authz.RequirePermission("System.Authority.User.ListUser"), h.ListUsers)
	rg.POST("/modifyUser", authz.RequirePermission("System.Authority.User.ModifyUser"), h.UpdateUser)
	rg.POST("/deleteUser", authz.RequirePermission("System.Authority.User.DeleteUser"), h.DeleteUser)
	rg.POST("/kickOutUser", authz.RequirePermission("System.Authority.User.KickOutUser"), h.KickOutUser)
	rg.POST("/unlockUser", authz.RequirePermission("System.Authority.User.EnableUser"), h.UnlockUser)
	rg.POST("/modifyPassword", h.ModifyPassword)
	rg.POST("/enableUser", authz.RequirePermission("System.Authority.User.EnableUser"), h.EnableUser)
	rg.POST("/disableUser", authz.RequirePermission("System.Authority.User.DisableUser"), h.DisableUser)
	rg.POST("/resetPassword", authz.RequirePermission("System.Authority.User.ResetPassword"), h.ResetPassword)
	rg.POST("/resetPasswordByLink", h.ResetPasswordByLink)
	rg.POST("/setTenancyForUser", h.SetTenancyForUser)
	rg.POST("/getLoginFailedTimes", h.GetLoginFailedTimes)
	rg.GET("/needChangePassword", h.NeedChangePassword)
	// TODO: rg.POST("/updateUserRole", h.UpdateUserRole) // handler not yet implemented
	// TODO: rg.POST("/getRoleAndTenantInfo", h.GetRoleAndTenantInfo) // handler not yet implemented

	// 角色管理 - Java: RoleManagementController
	rg.POST("/addRole", authz.RequirePermission("System.Authority.Role.AddRole"), h.CreateRole)
	rg.POST("/updateRole", authz.RequirePermission("System.Authority.Role.UpdateRole"), h.UpdateRole)
	rg.POST("/deleteRole", authz.RequirePermission("System.Authority.Role.DeleteRole"), h.DeleteRole)
	rg.POST("/listRole", authz.RequirePermission("System.Authority.Role.ListRole"), h.ListRoles)
	rg.POST("/listRolePermissionIds", authz.RequirePermission("System.Authority.Role.GetRolePermissions"), h.GetRolePermissions)
	rg.POST("/updateRolePermission", authz.RequirePermission("System.Authority.Role.UpdateRolePermissions"), h.UpdateRolePermissions)
	// Note: getPermission and getPermissionIdsForUser are registered in the authz module

	// 权限导出 - Java: PermissionExportController
	// TODO: rg.POST("/exportPermissionConfig", h.ExportPermissionConfig) // handler not yet implemented

	// Login logs (Go-specific, no direct Java controller counterpart found)
	rg.POST("/listLoginLog", authz.RequirePermission("System.Log.ListLoginLog"), h.ListLoginLogs)
}

// RegisterPublicRoutes registers public routes (no auth required) on the given router group.
// Aligned with Java AuthController (no class-level @RequestMapping) and VerificationCodeController
func RegisterPublicRoutes(rg *gin.RouterGroup, h *Handler) {
	// Java: AuthController - login/logout
	rg.POST("/uaa/oauth/token", h.Login)
	rg.DELETE("/uaa/exit", h.Logout)
	// Java: AuthController - token renewal (GET, root-level)
	rg.GET("/renewToken", h.RenewToken)
	// TODO: rg.POST("/uaa/auth/token", h.LoginForRestApi) // handler not yet implemented (northbound REST login)
	// Java: VerificationCodeController - captcha
	rg.GET("/v2/captchaImage", h.CaptchaImage)
}
