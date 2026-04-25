package store

import (
	"network-plan/internal/model"

	"gorm.io/gorm"
)

// AuditRepo 审计日志数据访问层
type AuditRepo struct {
	db *gorm.DB
}

// NewAuditRepo 创建审计日志 Repo 实例
func NewAuditRepo(db *gorm.DB) *AuditRepo {
	return &AuditRepo{db: db}
}

// Create 写入一条审计日志
func (r *AuditRepo) Create(log *model.AuditLog) error {
	return r.db.Create(log).Error
}

// ListAll 获取全部审计日志（按时间倒序）
func (r *AuditRepo) ListAll() ([]model.AuditLog, error) {
	var logs []model.AuditLog
	err := r.db.Order("created_at DESC").Find(&logs).Error
	return logs, err
}

// List 分页查询审计日志，可按 action 类型筛选。
// 返回值: 日志列表, 总记录数, 错误
func (r *AuditRepo) List(action string, page, pageSize int) ([]model.AuditLog, int64, error) {
	var logs []model.AuditLog
	var total int64

	q := r.db.Model(&model.AuditLog{})
	if action != "" {
		q = q.Where("action = ?", action)
	}
	q.Count(&total)

	err := q.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&logs).Error
	return logs, total, err
}

// WithTenant 返回按租户隔离的 AuditRepo 副本
func (r *AuditRepo) WithTenant(tenantID string) *AuditRepo {
	return &AuditRepo{db: r.db.Scopes(TenantScope(tenantID))}
}
