package authz

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/permissions", GetPermission)
	rg.GET("/permissions/user-ids", GetPermissionIdsForUser)
}
