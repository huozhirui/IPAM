package store

import (
	"network-plan/internal/model"

	"gorm.io/gorm"
)

// TenantRepo 租户数据访问层
type TenantRepo struct {
	db *gorm.DB
}

func NewTenantRepo(db *gorm.DB) *TenantRepo {
	return &TenantRepo{db: db}
}

// ListAll 获取全部租户
func (r *TenantRepo) ListAll() ([]model.Tenant, error) {
	var tenants []model.Tenant
	err := r.db.Order("id ASC").Find(&tenants).Error
	return tenants, err
}

// Create 创建租户
func (r *TenantRepo) Create(t *model.Tenant) error {
	return r.db.Create(t).Error
}

// FindBySlug 按标识查找租户
func (r *TenantRepo) FindBySlug(slug string) (*model.Tenant, error) {
	var t model.Tenant
	err := r.db.Where("slug = ?", slug).First(&t).Error
	return &t, err
}

// DeleteBySlug 按标识删除租户
func (r *TenantRepo) DeleteBySlug(slug string) error {
	return r.db.Where("slug = ?", slug).Delete(&model.Tenant{}).Error
}
