package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/netip"
	"network-plan/internal/ipam"
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
type AllocateReq struct {
	PoolID      uint64 `json:"pool_id" binding:"required"`  // 目标网段池 ID
	IPCount     int    `json:"ip_count" binding:"required"`  // 需要的 IP 数量
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

	var req UpdateAllocReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if _, err := h.allocRepo.GetByID(id); err != nil {
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

	if err := h.allocRepo.Update(id, fields); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	alloc, _ := h.allocRepo.GetByID(id)
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

	if req.AllocatedBy == "" {
		req.AllocatedBy = c.GetString("username")
	}
	alloc, err := h.doAllocate(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	detail, _ := json.Marshal(alloc)
	h.auditRepo.Create(&model.AuditLog{
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

// BatchAllocate 批量分配子网（逐条分配，任一失败则全部回滚）
// POST /api/allocations/batch
func (h *AllocationHandler) BatchAllocate(c *gin.Context) {
	var req BatchAllocateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	username := c.GetString("username")
	var results []model.Allocation
	for i, item := range req.Items {
		if item.AllocatedBy == "" {
			item.AllocatedBy = username
		}
		alloc, err := h.doAllocate(item)
		if err != nil {
			for _, a := range results {
				h.allocRepo.Delete(a.ID)
			}
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "index": i})
			return
		}
		results = append(results, *alloc)
	}

	for _, a := range results {
		detail, _ := json.Marshal(a)
		h.auditRepo.Create(&model.AuditLog{
			Action:   "ALLOCATE",
			Detail:   string(detail),
			Operator: c.GetString("username"),
		})
	}

	c.JSON(http.StatusOK, results)
}

// getPoolAllocated 获取池的 PoolRange 和已分配前缀列表（复用逻辑）
func (h *AllocationHandler) getPoolAllocated(poolID uint64) (*model.IPPool, ipam.PoolRange, []netip.Prefix, error) {
	pool, err := h.poolRepo.GetByID(poolID)
	if err != nil {
		return nil, ipam.PoolRange{}, nil, fmt.Errorf("pool not found: %w", err)
	}

	pr, err := poolRangeFromModel(pool)
	if err != nil {
		return nil, ipam.PoolRange{}, nil, err
	}

	existing, err := h.allocRepo.ListByPoolID(poolID)
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
func (h *AllocationHandler) doAllocate(req AllocateReq) (*model.Allocation, error) {
	_, pr, allocated, err := h.getPoolAllocated(req.PoolID)
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
		PoolID:      req.PoolID,
		CIDR:        result.String(),
		IPCount:     req.IPCount,
		ActualCount: actualIPs,
		Purpose:     req.Purpose,
		AllocatedBy: req.AllocatedBy,
	}
	if err := h.allocRepo.Create(alloc); err != nil {
		return nil, err
	}

	return alloc, nil
}

// List 查询分配记录，支持按 pool_id 筛选
// GET /api/allocations?pool_id=X
func (h *AllocationHandler) List(c *gin.Context) {
	poolIDStr := c.Query("pool_id")

	var allocs []model.Allocation
	var err error
	if poolIDStr != "" {
		poolID, _ := strconv.ParseUint(poolIDStr, 10, 64)
		allocs, err = h.allocRepo.ListByPoolID(poolID)
	} else {
		allocs, err = h.allocRepo.ListAll()
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

	alloc, err := h.allocRepo.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "allocation not found"})
		return
	}

	if err := h.allocRepo.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	detail, _ := json.Marshal(alloc)
	h.auditRepo.Create(&model.AuditLog{
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

	_, pr, allocated, err := h.getPoolAllocated(id)
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

	_, pr, allocated, err := h.getPoolAllocated(req.PoolID)
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
