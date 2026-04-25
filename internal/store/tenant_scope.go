package store

import "gorm.io/gorm"

// TenantScope 返回按 tenant_id 过滤的 GORM Scope。
func TenantScope(tenantID string) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("tenant_id = ?", tenantID)
	}
}
