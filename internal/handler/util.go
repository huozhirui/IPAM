package handler

import (
	"network-plan/internal/ipam"
	"network-plan/internal/model"
	"strconv"
)

// parseUint64 将字符串转为 uint64，失败返回 0
func parseUint64(s string) uint64 {
	v, _ := strconv.ParseUint(s, 10, 64)
	return v
}

// poolRangeFromModel 从数据库 IPPool 模型构建 PoolRange。
// 优先使用 start_ip/end_ip（精确范围），fallback 到 CIDR。
func poolRangeFromModel(p *model.IPPool) (ipam.PoolRange, error) {
	if p.StartIP != "" && p.EndIP != "" {
		return ipam.PoolRangeFromIPs(p.StartIP, p.EndIP)
	}
	return ipam.PoolRangeFromCIDR(p.CIDR)
}
