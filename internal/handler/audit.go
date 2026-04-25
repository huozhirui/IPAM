package handler

import (
	"net/http"
	"network-plan/internal/middleware"
	"network-plan/internal/store"
	"strconv"

	"github.com/gin-gonic/gin"
)

// AuditHandler 审计日志相关 API
type AuditHandler struct {
	auditRepo *store.AuditRepo
}

// NewAuditHandler 创建审计日志 Handler
func NewAuditHandler(ar *store.AuditRepo) *AuditHandler {
	return &AuditHandler{auditRepo: ar}
}

// List 分页查询审计日志
// GET /api/audit?action=ALLOCATE&page=1&page_size=20
func (h *AuditHandler) List(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	auditRepo := h.auditRepo.WithTenant(tenantID)

	action := c.Query("action")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	logs, total, err := auditRepo.List(action, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"items": logs,
		"total": total,
		"page":  page,
	})
}
