package handler

import (
	"net/http"
	"network-plan/internal/model"
	"network-plan/internal/store"

	"github.com/gin-gonic/gin"
)

// TenantHandler 租户相关 API
type TenantHandler struct {
	tenantRepo *store.TenantRepo
	userRepo   *store.UserRepo
}

func NewTenantHandler(tr *store.TenantRepo, ur *store.UserRepo) *TenantHandler {
	return &TenantHandler{tenantRepo: tr, userRepo: ur}
}

// ListAll 获取所有租户（公开接口，供登录页下拉用）
// GET /api/tenants
func (h *TenantHandler) ListAll(c *gin.Context) {
	tenants, err := h.tenantRepo.ListAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, tenants)
}

// ListMyTenants 获取当前用户可访问的租户列表（需认证）
// GET /api/my-tenants
func (h *TenantHandler) ListMyTenants(c *gin.Context) {
	username, _ := c.Get("username")
	tenantIDs, err := h.userRepo.ListTenantsByUsername(username.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, tenantIDs)
}

// Create 创建租户（管理员）
// POST /api/tenants
func (h *TenantHandler) Create(c *gin.Context) {
	var req struct {
		Name string `json:"name" binding:"required"`
		Slug string `json:"slug" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name 和 slug 为必填项"})
		return
	}
	tenant := &model.Tenant{Name: req.Name, Slug: req.Slug}
	if err := h.tenantRepo.Create(tenant); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "租户已存在或创建失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusCreated, tenant)
}

// Delete 删除租户（管理员）
// DELETE /api/tenants/:slug
func (h *TenantHandler) Delete(c *gin.Context) {
	slug := c.Param("slug")
	if slug == "default" {
		c.JSON(http.StatusForbidden, gin.H{"error": "不能删除默认租户"})
		return
	}
	if _, err := h.tenantRepo.FindBySlug(slug); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "租户不存在"})
		return
	}
	if err := h.tenantRepo.DeleteBySlug(slug); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "租户已删除"})
}
