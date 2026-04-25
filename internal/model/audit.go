package model

import "time"

// AuditLog 操作审计日志，记录所有分配、回收、增删池等操作。
type AuditLog struct {
	ID        uint64    `json:"id" gorm:"primaryKey;autoIncrement"`
	TenantID  string    `json:"tenant_id" gorm:"type:varchar(64);not null;default:'default';index"`
	Action    string    `json:"action" gorm:"type:varchar(32);index;not null"` // 操作类型: ALLOCATE / RECLAIM / CREATE_POOL / DELETE_POOL
	Detail    string    `json:"detail" gorm:"type:text"`                       // 操作详情 (JSON)
	Operator  string    `json:"operator" gorm:"type:varchar(128);default:''"` // 操作人
	CreatedAt time.Time `json:"created_at" gorm:"index"`
}

func (AuditLog) TableName() string { return "audit_log" }
