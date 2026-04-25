package model

import "time"

// Allocation 子网分配记录，表示从某个网段池中划出的一段子网。
type Allocation struct {
	ID          uint64    `json:"id" gorm:"primaryKey;autoIncrement"`
	TenantID    string    `json:"tenant_id" gorm:"type:varchar(64);not null;default:'default';uniqueIndex:idx_tenant_alloc_cidr;index"`
	PoolID      uint64    `json:"pool_id" gorm:"index;not null"`                       // 所属网段池 ID
	CIDR        string    `json:"cidr" gorm:"column:cidr;type:varchar(43);not null;uniqueIndex:idx_tenant_alloc_cidr"` // 分配的子网 CIDR
	IPCount     int       `json:"ip_count" gorm:"not null"`                             // 用户申请的 IP 数量
	ActualCount int       `json:"actual_count" gorm:"not null"`                         // 实际分配 IP 数（向上对齐到 2^n）
	Purpose     string    `json:"purpose" gorm:"type:varchar(256);not null"`            // 用途标签（VPC/业务线/环境等）
	AllocatedBy string    `json:"allocated_by" gorm:"type:varchar(128);default:''"`     // 操作人
	AllocatedAt time.Time `json:"allocated_at" gorm:"autoCreateTime"`                   // 分配时间
}

func (Allocation) TableName() string { return "allocation" }
