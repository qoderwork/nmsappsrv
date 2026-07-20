package authz

import (
	"github.com/gin-gonic/gin"

	"nmsappsrv/internal/middleware"
	"nmsappsrv/pkg/utils"
)

// GetPermissionIdsForUser handles GET /api/v1/auth/permissions/user-ids.
// It returns the effective permission ids for the currently authenticated
// user, mirroring Java RoleManagementServiceImpl.getPermissionIdsForUser.
func GetPermissionIdsForUser(c *gin.Context) {
	roleNames := middleware.GetRoleNames(c)
	vo := CurrentUserPermissionIDs(roleNames)
	utils.Success(c, vo)
}

// GetPermission handles GET /api/v1/auth/permissions.
// It returns the full permission registry (the permission tree the frontend
// uses to render the role-permission matrix), mirroring Java
// RoleManagementServiceImpl.getPermission.
func GetPermission(c *gin.Context) {
	utils.Success(c, PermissionRegistry())
}
