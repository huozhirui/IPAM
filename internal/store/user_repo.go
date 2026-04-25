package store

import (
	"network-plan/internal/model"

	"gorm.io/gorm"
)

// UserRepo 用户数据访问层
type UserRepo struct {
	db *gorm.DB
}

func NewUserRepo(db *gorm.DB) *UserRepo {
	return &UserRepo{db: db}
}

func (r *UserRepo) Create(user *model.User) error {
	return r.db.Create(user).Error
}

func (r *UserRepo) FindByUsername(username string) (*model.User, error) {
	var user model.User
	err := r.db.Where("username = ?", username).First(&user).Error
	return &user, err
}

func (r *UserRepo) FindByID(id uint64) (*model.User, error) {
	var user model.User
	err := r.db.First(&user, id).Error
	return &user, err
}

func (r *UserRepo) Count() (int64, error) {
	var count int64
	err := r.db.Model(&model.User{}).Count(&count).Error
	return count, err
}

func (r *UserRepo) List() ([]model.User, error) {
	var users []model.User
	err := r.db.Order("id ASC").Find(&users).Error
	return users, err
}

func (r *UserRepo) UpdatePassword(username, passwordHash string) error {
	return r.db.Model(&model.User{}).Where("username = ?", username).
		Update("password_hash", passwordHash).Error
}

func (r *UserRepo) DeleteByUsername(username string) error {
	return r.db.Where("username = ?", username).Delete(&model.User{}).Error
}

// WithTenant 返回按租户隔离的 UserRepo 副本
func (r *UserRepo) WithTenant(tenantID string) *UserRepo {
	return &UserRepo{db: r.db.Scopes(TenantScope(tenantID))}
}

// ListTenantsByUsername 查询某用户名存在于哪些租户
func (r *UserRepo) ListTenantsByUsername(username string) ([]string, error) {
	var tenantIDs []string
	err := r.db.Model(&model.User{}).
		Where("username = ?", username).
		Distinct("tenant_id").
		Pluck("tenant_id", &tenantIDs).Error
	return tenantIDs, err
}
