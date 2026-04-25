package model

import "time"

// 用户角色常量
const (
	RoleAdmin = "admin" // 管理员：可增删网段池、管理用户
	RoleUser  = "user"  // 普通用户：可分配子网、查看数据
)

// User 用户表
type User struct {
	ID           uint64    `json:"id" gorm:"primaryKey;autoIncrement"`
	TenantID     string    `json:"tenant_id" gorm:"type:varchar(64);not null;default:'default';uniqueIndex:idx_tenant_username;index"`
	Username     string    `json:"username" gorm:"type:varchar(64);not null;uniqueIndex:idx_tenant_username"`
	PasswordHash string    `json:"-" gorm:"type:varchar(255);not null"`
	Role         string    `json:"role" gorm:"type:varchar(16);not null;default:user"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (User) TableName() string { return "user" }
