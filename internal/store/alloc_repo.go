package store

import (
	"network-plan/internal/model"

	"gorm.io/gorm"
)

// AllocRepo 子网分配记录数据访问层
type AllocRepo struct {
	db *gorm.DB
}

// NewAllocRepo 创建分配记录 Repo 实例
func NewAllocRepo(db *gorm.DB) *AllocRepo {
	return &AllocRepo{db: db}
}

// Create 新增一条分配记录
func (r *AllocRepo) Create(alloc *model.Allocation) error {
	return r.db.Create(alloc).Error
}

// CreateBatch 批量新增分配记录（事务内执行）
func (r *AllocRepo) CreateBatch(allocs []model.Allocation) error {
	return r.db.Create(&allocs).Error
}

// ListByPoolID 按网段池 ID 查询其下所有分配记录
func (r *AllocRepo) ListByPoolID(poolID uint64) ([]model.Allocation, error) {
	var allocs []model.Allocation
	err := r.db.Where("pool_id = ?", poolID).Order("allocated_at DESC").Find(&allocs).Error
	return allocs, err
}

// ListAll 获取全部分配记录（按时间倒序）
func (r *AllocRepo) ListAll() ([]model.Allocation, error) {
	var allocs []model.Allocation
	err := r.db.Order("allocated_at DESC").Find(&allocs).Error
	return allocs, err
}

// GetByID 根据 ID 获取单条分配记录
func (r *AllocRepo) GetByID(id uint64) (*model.Allocation, error) {
	var alloc model.Allocation
	err := r.db.First(&alloc, id).Error
	return &alloc, err
}

// Delete 删除分配记录（即回收子网）
func (r *AllocRepo) Delete(id uint64) error {
	return r.db.Delete(&model.Allocation{}, id).Error
}

// CountByPoolID 统计某网段池下的分配记录数
func (r *AllocRepo) CountByPoolID(poolID uint64) (int64, error) {
	var count int64
	err := r.db.Model(&model.Allocation{}).Where("pool_id = ?", poolID).Count(&count).Error
	return count, err
}

// Update 更新分配记录的用途和负责人
func (r *AllocRepo) Update(id uint64, fields map[string]interface{}) error {
	return r.db.Model(&model.Allocation{}).Where("id = ?", id).Updates(fields).Error
}

// SumActualCountByPoolID 统计某网段池已分配的实际 IP 总数
func (r *AllocRepo) SumActualCountByPoolID(poolID uint64) (int64, error) {
	var sum int64
	row := r.db.Model(&model.Allocation{}).Where("pool_id = ?", poolID).Select("COALESCE(SUM(actual_count),0)").Row()
	err := row.Scan(&sum)
	return sum, err
}

// WithTenant 返回按租户隔离的 AllocRepo 副本
func (r *AllocRepo) WithTenant(tenantID string) *AllocRepo {
	return &AllocRepo{db: r.db.Scopes(TenantScope(tenantID))}
}
