package tenancy

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	rg.POST("/addTenancy", h.AddTenancy)
	rg.POST("/updateTenancy", h.UpdateTenancy)
	rg.POST("/listTenancy", h.ListTenancy)
	rg.POST("/deleteTenancy", h.DeleteTenancy)
	rg.POST("/viewTenancy", h.ViewTenancy)
}