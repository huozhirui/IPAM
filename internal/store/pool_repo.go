package store

import (
	"network-plan/internal/model"

	"gorm.io/gorm"
)

// PoolRepo 网段池数据访问层
type PoolRepo struct {
	db *gorm.DB
}

// NewPoolRepo 创建网段池 Repo 实例
func NewPoolRepo(db *gorm.DB) *PoolRepo {
	return &PoolRepo{db: db}
}

// Create 新增一个网段池
func (r *PoolRepo) Create(pool *model.IPPool) error {
	return r.db.Create(pool).Error
}

// List 按创建时间倒序获取所有网段池
func (r *PoolRepo) List() ([]model.IPPool, error) {
	var pools []model.IPPool
	err := r.db.Order("created_at DESC").Find(&pools).Error
	return pools, err
}

// GetByID 根据 ID 获取单个网段池
func (r *PoolRepo) GetByID(id uint64) (*model.IPPool, error) {
	var pool model.IPPool
	err := r.db.First(&pool, id).Error
	return &pool, err
}

// Delete 根据 ID 删除网段池（调用前应确认无关联分配）
func (r *PoolRepo) Delete(id uint64) error {
	return r.db.Delete(&model.IPPool{}, id).Error
}
