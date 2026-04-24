package handler

import (
	"net/http"
	"network-plan/internal/store"

	"github.com/gin-gonic/gin"
)

// DashboardHandler 仪表盘 API
type DashboardHandler struct {
	poolRepo  *store.PoolRepo
	allocRepo *store.AllocRepo
}

// NewDashboardHandler 创建仪表盘 Handler
func NewDashboardHandler(pr *store.PoolRepo, ar *store.AllocRepo) *DashboardHandler {
	return &DashboardHandler{poolRepo: pr, allocRepo: ar}
}

// DashboardResp 仪表盘响应数据
type DashboardResp struct {
	PoolCount       int            `json:"pool_count"`        // 网段池总数
	AllocationCount int            `json:"allocation_count"`  // 已分配子网数
	TotalIPs        uint64         `json:"total_ips"`         // 全部网段池的 IP 总数
	UsedIPs         int64          `json:"used_ips"`          // 已分配 IP 总数
	UsageRate       float64        `json:"usage_rate"`        // 总使用率百分比
	RecentAllocs    []RecentAlloc  `json:"recent_allocs"`     // 最近 5 条分配记录
}

// RecentAlloc 最近分配记录摘要
type RecentAlloc struct {
	CIDR        string `json:"cidr"`
	Purpose     string `json:"purpose"`
	ActualCount int    `json:"actual_count"`
	AllocatedAt string `json:"allocated_at"`
}

// Get 获取仪表盘汇总数据
// GET /api/dashboard
func (h *DashboardHandler) Get(c *gin.Context) {
	pools, err := h.poolRepo.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var totalIPs uint64
	var usedIPs int64
	for _, p := range pools {
		pr, err := poolRangeFromModel(&p)
		if err != nil {
			continue
		}
		totalIPs += pr.IPCount()
		used, _ := h.allocRepo.SumActualCountByPoolID(p.ID)
		usedIPs += used
	}

	allAllocs, _ := h.allocRepo.ListAll()
	rate := 0.0
	if totalIPs > 0 {
		rate = float64(usedIPs) / float64(totalIPs) * 100
	}

	// 取最近 5 条
	recent := make([]RecentAlloc, 0)
	limit := 5
	if len(allAllocs) < limit {
		limit = len(allAllocs)
	}
	for _, a := range allAllocs[:limit] {
		recent = append(recent, RecentAlloc{
			CIDR:        a.CIDR,
			Purpose:     a.Purpose,
			ActualCount: a.ActualCount,
			AllocatedAt: a.AllocatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	c.JSON(http.StatusOK, DashboardResp{
		PoolCount:       len(pools),
		AllocationCount: len(allAllocs),
		TotalIPs:        totalIPs,
		UsedIPs:         usedIPs,
		UsageRate:       rate,
		RecentAllocs:    recent,
	})
}
