package authz

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/getPermission", GetPermission)
	rg.POST("/getPermissionIdsForUser", GetPermissionIdsForUser)
}