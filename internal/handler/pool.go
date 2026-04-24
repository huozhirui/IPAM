// Package handler 提供 HTTP API 请求处理函数（Gin handlers）。
package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"network-plan/internal/ipam"
	"network-plan/internal/model"
	"network-plan/internal/store"

	"github.com/gin-gonic/gin"
)

// PoolHandler 网段池相关 API
type PoolHandler struct {
	poolRepo  *store.PoolRepo
	allocRepo *store.AllocRepo
	auditRepo *store.AuditRepo
}

// NewPoolHandler 创建网段池 Handler
func NewPoolHandler(pr *store.PoolRepo, ar *store.AllocRepo, aur *store.AuditRepo) *PoolHandler {
	return &PoolHandler{poolRepo: pr, allocRepo: ar, auditRepo: aur}
}

// CreatePoolReq 创建网段池请求体，支持两种模式：
//   - CIDR 模式: 填 cidr 字段（如 "10.0.0.0/16"）
//   - 范围模式: 填 start_ip + end_ip 字段（如 "10.0.0.0" + "10.0.255.255"）
type CreatePoolReq struct {
	CIDR        string `json:"cidr"`                           // CIDR 模式
	StartIP     string `json:"start_ip"`                       // 范围模式：起始 IP
	EndIP       string `json:"end_ip"`                         // 范围模式：结束 IP
	Name        string `json:"name" binding:"required"`        // 网段池名称
	Description string `json:"description"`                    // 备注
}

// Create 新增网段池
// POST /api/pools
func (h *PoolHandler) Create(c *gin.Context) {
	var req CreatePoolReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	pool := &model.IPPool{
		Name:        req.Name,
		Description: req.Description,
	}

	if req.CIDR != "" {
		// CIDR 模式
		if err := ipam.ValidateCIDR(req.CIDR); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		p, _ := ipam.ParseCIDR(req.CIDR)
		pr, _ := ipam.PoolRangeFromCIDR(p.String())
		pool.CIDR = p.String()
		pool.StartIP = ipam.Uint32ToAddr(pr.Start).String()
		pool.EndIP = ipam.Uint32ToAddr(pr.End - 1).String() // 存储闭区间
	} else if req.StartIP != "" && req.EndIP != "" {
		// 范围模式
		_, err := ipam.PoolRangeFromIPs(req.StartIP, req.EndIP)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		pool.CIDR = fmt.Sprintf("%s - %s", req.StartIP, req.EndIP)
		pool.StartIP = req.StartIP
		pool.EndIP = req.EndIP
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "must provide either 'cidr' or 'start_ip'+'end_ip'"})
		return
	}

	if err := h.poolRepo.Create(pool); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	detail, _ := json.Marshal(pool)
	operator, _ := c.Get("username")
	h.auditRepo.Create(&model.AuditLog{
		Action:   "CREATE_POOL",
		Detail:   string(detail),
		Operator: operator.(string),
	})

	c.JSON(http.StatusOK, pool)
}

// PoolWithUsage 带使用率统计的网段池信息
type PoolWithUsage struct {
	model.IPPool
	TotalIPs   uint64  `json:"total_ips"`   // 网段池总 IP 数
	UsedIPs    int64   `json:"used_ips"`     // 已分配 IP 数
	UsageRate  float64 `json:"usage_rate"`   // 使用率百分比
	AllocCount int64   `json:"alloc_count"`  // 分配记录数
}

// List 获取所有网段池（含使用率统计）
// GET /api/pools
func (h *PoolHandler) List(c *gin.Context) {
	pools, err := h.poolRepo.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var result []PoolWithUsage
	for _, p := range pools {
		pr, err := poolRangeFromModel(&p)
		if err != nil {
			continue
		}
		totalIPs := pr.IPCount()
		usedIPs, _ := h.allocRepo.SumActualCountByPoolID(p.ID)
		allocCount, _ := h.allocRepo.CountByPoolID(p.ID)

		rate := 0.0
		if totalIPs > 0 {
			rate = float64(usedIPs) / float64(totalIPs) * 100
		}
		result = append(result, PoolWithUsage{
			IPPool:     p,
			TotalIPs:   totalIPs,
			UsedIPs:    usedIPs,
			UsageRate:  rate,
			AllocCount: allocCount,
		})
	}

	c.JSON(http.StatusOK, result)
}

// Delete 删除网段池（有分配记录时拒绝删除）
// DELETE /api/pools/:id
func (h *PoolHandler) Delete(c *gin.Context) {
	id := parseUint64(c.Param("id"))
	if id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	count, _ := h.allocRepo.CountByPoolID(id)
	if count > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "pool has active allocations, please reclaim them first"})
		return
	}

	pool, err := h.poolRepo.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "pool not found"})
		return
	}

	if err := h.poolRepo.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	detail, _ := json.Marshal(pool)
	operator, _ := c.Get("username")
	h.auditRepo.Create(&model.AuditLog{
		Action:   "DELETE_POOL",
		Detail:   string(detail),
		Operator: operator.(string),
	})

	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}
