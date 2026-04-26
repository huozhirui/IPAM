package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/netip"
	"network-plan/internal/ipam"
	"network-plan/internal/middleware"
	"network-plan/internal/model"
	"network-plan/internal/store"
	"strconv"

	"github.com/gin-gonic/gin"
)

// AllocationHandler 子网分配相关 API
type AllocationHandler struct {
	poolRepo  *store.PoolRepo
	allocRepo *store.AllocRepo
	auditRepo *store.AuditRepo
}

// NewAllocationHandler 创建分配 Handler
func NewAllocationHandler(pr *store.PoolRepo, ar *store.AllocRepo, aur *store.AuditRepo) *AllocationHandler {
	return &AllocationHandler{poolRepo: pr, allocRepo: ar, auditRepo: aur}
}

// AllocateReq 单条分配请求体
// 支持两种模式：
//   1. 按数量分配：指定 ip_count，系统自动选择最佳 CIDR
//   2. 按 CIDR 分配：指定 cidr，系统验证后直接分配指定网段
type AllocateReq struct {
	PoolID      uint64 `json:"pool_id" binding:"required"`  // 目标网段池 ID
	IPCount     int    `json:"ip_count"`                     // 需要的 IP 数量（按数量模式）
	CIDR        string `json:"cidr"`                         // 指定 CIDR 地址（按 CIDR 模式）
	Purpose     string `json:"purpose" binding:"required"`   // 用途标签
	AllocatedBy string `json:"allocated_by"`                 // 负责人
}

// UpdateAllocReq 编辑分配记录请求体
type UpdateAllocReq struct {
	Purpose     string `json:"purpose"`
	AllocatedBy string `json:"allocated_by"`
}

// Update 修改分配记录的用途和负责人
// PUT /api/allocations/:id
func (h *AllocationHandler) Update(c *gin.Context) {
	id := parseUint64(c.Param("id"))
	if id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	tenantID := middleware.GetTenantID(c)
	allocRepo := h.allocRepo.WithTenant(tenantID)

	var req UpdateAllocReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if _, err := allocRepo.GetByID(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "allocation not found"})
		return
	}

	fields := map[string]interface{}{}
	if req.Purpose != "" {
		fields["purpose"] = req.Purpose
	}
	if req.AllocatedBy != "" {
		fields["allocated_by"] = req.AllocatedBy
	}
	if len(fields) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "nothing to update"})
		return
	}

	if err := allocRepo.Update(id, fields); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	alloc, _ := allocRepo.GetByID(id)
	c.JSON(http.StatusOK, alloc)
}

// Allocate 从指定网段池中分配一个子网
// POST /api/allocations
func (h *AllocationHandler) Allocate(c *gin.Context) {
	var req AllocateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenantID := middleware.GetTenantID(c)
	poolRepo := h.poolRepo.WithTenant(tenantID)
	allocRepo := h.allocRepo.WithTenant(tenantID)
	auditRepo := h.auditRepo.WithTenant(tenantID)

	if req.IPCount == 0 && req.CIDR == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ip_count or cidr is required"})
		return
	}

	if req.AllocatedBy == "" {
		req.AllocatedBy = c.GetString("username")
	}

	var alloc *model.Allocation
	var err error
	if req.CIDR != "" {
		alloc, err = doAllocateByCIDR(poolRepo, allocRepo, tenantID, req)
	} else {
		alloc, err = doAllocate(poolRepo, allocRepo, tenantID, req)
	}
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	detail, _ := json.Marshal(alloc)
	auditRepo.Create(&model.AuditLog{
		TenantID: tenantID,
		Action:   "ALLOCATE",
		Detail:   string(detail),
		Operator: c.GetString("username"),
	})

	c.JSON(http.StatusOK, alloc)
}

// BatchAllocateReq 批量分配请求体
type BatchAllocateReq struct {
	Items []AllocateReq `json:"items" binding:"required,min=1"`
}

// BatchItemResult 批量分配逐条结果
type BatchItemResult struct {
	Index   int               `json:"index"`
	Success bool              `json:"success"`
	Error   string            `json:"error,omitempty"`
	Alloc   *model.Allocation `json:"allocation,omitempty"`
}

// BatchAllocate 批量分配子网（逐条处理，返回每条结果）
// POST /api/allocations/batch
func (h *AllocationHandler) BatchAllocate(c *gin.Context) {
	var req BatchAllocateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenantID := middleware.GetTenantID(c)
	poolRepo := h.poolRepo.WithTenant(tenantID)
	allocRepo := h.allocRepo.WithTenant(tenantID)
	auditRepo := h.auditRepo.WithTenant(tenantID)

	username := c.GetString("username")
	results := make([]BatchItemResult, len(req.Items))
	successCount := 0

	for i, item := range req.Items {
		if item.AllocatedBy == "" {
			item.AllocatedBy = username
		}

		if item.IPCount == 0 && item.CIDR == "" {
			results[i] = BatchItemResult{Index: i, Success: false, Error: "ip_count or cidr is required"}
			continue
		}

		var alloc *model.Allocation
		var err error
		if item.CIDR != "" {
			alloc, err = doAllocateByCIDR(poolRepo, allocRepo, tenantID, item)
		} else {
			alloc, err = doAllocate(poolRepo, allocRepo, tenantID, item)
		}

		if err != nil {
			results[i] = BatchItemResult{Index: i, Success: false, Error: err.Error()}
		} else {
			results[i] = BatchItemResult{Index: i, Success: true, Alloc: alloc}
			successCount++

			detail, _ := json.Marshal(alloc)
			auditRepo.Create(&model.AuditLog{
				TenantID: tenantID,
				Action:   "ALLOCATE",
				Detail:   string(detail),
				Operator: username,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"results":       results,
		"total":         len(req.Items),
		"success_count": successCount,
		"fail_count":    len(req.Items) - successCount,
	})
}

// getPoolAllocated 获取池的 PoolRange 和已分配前缀列表（复用逻辑）
func getPoolAllocated(poolRepo *store.PoolRepo, allocRepo *store.AllocRepo, poolID uint64) (*model.IPPool, ipam.PoolRange, []netip.Prefix, error) {
	pool, err := poolRepo.GetByID(poolID)
	if err != nil {
		return nil, ipam.PoolRange{}, nil, fmt.Errorf("pool not found: %w", err)
	}

	pr, err := poolRangeFromModel(pool)
	if err != nil {
		return nil, ipam.PoolRange{}, nil, err
	}

	existing, err := allocRepo.ListByPoolID(poolID)
	if err != nil {
		return nil, ipam.PoolRange{}, nil, err
	}
	var allocated []netip.Prefix
	for _, a := range existing {
		p, err := ipam.ParseCIDR(a.CIDR)
		if err == nil {
			allocated = append(allocated, p)
		}
	}

	return pool, pr, allocated, nil
}

// doAllocate 执行单条分配的核心逻辑
func doAllocate(poolRepo *store.PoolRepo, allocRepo *store.AllocRepo, tenantID string, req AllocateReq) (*model.Allocation, error) {
	_, pr, allocated, err := getPoolAllocated(poolRepo, allocRepo, req.PoolID)
	if err != nil {
		return nil, err
	}

	// 调用 IPAM 核心算法分配子网
	result, actualIPs, err := ipam.AllocateSubnet(pr, allocated, req.IPCount)
	if err != nil {
		return nil, err
	}

	// 写入数据库
	alloc := &model.Allocation{
		TenantID:    tenantID,
		PoolID:      req.PoolID,
		CIDR:        result.String(),
		IPCount:     req.IPCount,
		ActualCount: actualIPs,
		Purpose:     req.Purpose,
		AllocatedBy: req.AllocatedBy,
	}
	if err := allocRepo.Create(alloc); err != nil {
		return nil, err
	}

	return alloc, nil
}

// doAllocateByCIDR 按指定 CIDR 地址分配子网
func doAllocateByCIDR(poolRepo *store.PoolRepo, allocRepo *store.AllocRepo, tenantID string, req AllocateReq) (*model.Allocation, error) {
	// 验证 CIDR 格式
	if err := ipam.ValidateCIDR(req.CIDR); err != nil {
		return nil, err
	}

	prefix, err := ipam.ParseCIDR(req.CIDR)
	if err != nil {
		return nil, err
	}

	pool, _, allocated, err := getPoolAllocated(poolRepo, allocRepo, req.PoolID)
	if err != nil {
		return nil, err
	}

	// 验证 CIDR 在池范围内
	if pool.CIDR != "" {
		if err := ipam.ValidateSubnetInPool(pool.CIDR, req.CIDR); err != nil {
			return nil, err
		}
	} else {
		// IP 范围模式的池，手动检查
		pr, err := poolRangeFromModel(pool)
		if err != nil {
			return nil, err
		}
		s, e := ipam.AddrToUint32(prefix.Addr()), ipam.AddrToUint32(prefix.Addr())+uint32(ipam.PrefixIPCount(prefix))
		if s < pr.Start || e > pr.End {
			return nil, fmt.Errorf("CIDR %s is not within pool range %s - %s", req.CIDR, ipam.Uint32ToAddr(pr.Start), ipam.Uint32ToAddr(pr.End-1))
		}
	}

	// 检查与已有分配的冲突
	var existingCIDRs []string
	for _, a := range allocated {
		existingCIDRs = append(existingCIDRs, a.String())
	}
	if err := ipam.CheckConflict(req.CIDR, existingCIDRs); err != nil {
		return nil, err
	}

	actualIPs := int(ipam.PrefixIPCount(prefix))

	alloc := &model.Allocation{
		TenantID:    tenantID,
		PoolID:      req.PoolID,
		CIDR:        prefix.String(),
		IPCount:     actualIPs,
		ActualCount: actualIPs,
		Purpose:     req.Purpose,
		AllocatedBy: req.AllocatedBy,
	}
	if err := allocRepo.Create(alloc); err != nil {
		return nil, err
	}

	return alloc, nil
}

// List 查询分配记录，支持多条件搜索
// GET /api/allocations?pool_id=X&cidr=10.0&purpose=prod&allocated_by=alice
func (h *AllocationHandler) List(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	allocRepo := h.allocRepo.WithTenant(tenantID)

	poolIDStr := c.Query("pool_id")
	cidr := c.Query("cidr")
	purpose := c.Query("purpose")
	allocatedBy := c.Query("allocated_by")

	var poolID uint64
	if poolIDStr != "" {
		poolID, _ = strconv.ParseUint(poolIDStr, 10, 64)
	}

	// 有任意搜索条件时走 Search，否则走 ListAll
	hasFilter := poolID > 0 || cidr != "" || purpose != "" || allocatedBy != ""

	var allocs []model.Allocation
	var err error
	if hasFilter {
		allocs, err = allocRepo.Search(poolID, cidr, purpose, allocatedBy)
	} else {
		allocs, err = allocRepo.ListAll()
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, allocs)
}

// Reclaim 回收子网（删除分配记录，释放 IP 回网段池）
// DELETE /api/allocations/:id
func (h *AllocationHandler) Reclaim(c *gin.Context) {
	id := parseUint64(c.Param("id"))
	if id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	tenantID := middleware.GetTenantID(c)
	allocRepo := h.allocRepo.WithTenant(tenantID)
	auditRepo := h.auditRepo.WithTenant(tenantID)

	alloc, err := allocRepo.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "allocation not found"})
		return
	}

	if err := allocRepo.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	detail, _ := json.Marshal(alloc)
	auditRepo.Create(&model.AuditLog{
		TenantID: tenantID,
		Action:   "RECLAIM",
		Detail:   string(detail),
		Operator: c.GetString("username"),
	})

	c.JSON(http.StatusOK, gin.H{"message": "reclaimed"})
}

// FreeBlocks 查询指定网段池的剩余可用网段
// GET /api/pools/:id/free-blocks
func (h *AllocationHandler) FreeBlocks(c *gin.Context) {
	id := parseUint64(c.Param("id"))
	if id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	tenantID := middleware.GetTenantID(c)
	poolRepo := h.poolRepo.WithTenant(tenantID)
	allocRepo := h.allocRepo.WithTenant(tenantID)

	_, pr, allocated, err := getPoolAllocated(poolRepo, allocRepo, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	blocks := ipam.FindFreeBlocks(pr, allocated)
	c.JSON(http.StatusOK, blocks)
}

// CalculateReq 预计算请求体
type CalculateReq struct {
	PoolID  uint64 `json:"pool_id" binding:"required"`
	IPCount int    `json:"ip_count" binding:"required"`
}

// CalculateResp 预计算响应
type CalculateResp struct {
	PrefixLen   int    `json:"prefix_len"`
	ActualCount int    `json:"actual_count"`
	SuggestCIDR string `json:"suggest_cidr"`
}

// Calculate 预计算：输入 IP 数量，返回推荐 CIDR（不实际分配）
// POST /api/calculate
func (h *AllocationHandler) Calculate(c *gin.Context) {
	var req CalculateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenantID := middleware.GetTenantID(c)
	poolRepo := h.poolRepo.WithTenant(tenantID)
	allocRepo := h.allocRepo.WithTenant(tenantID)

	_, pr, allocated, err := getPoolAllocated(poolRepo, allocRepo, req.PoolID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	result, actualIPs, err := ipam.AllocateSubnet(pr, allocated, req.IPCount)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, CalculateResp{
		PrefixLen:   result.Bits(),
		ActualCount: actualIPs,
		SuggestCIDR: result.String(),
	})
}
