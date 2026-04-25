package model

import "time"

// Tenant 租户，用于多租户数据隔离。
type Tenant struct {
	ID        uint64    `json:"id" gorm:"primaryKey;autoIncrement"`
	Name      string    `json:"name" gorm:"type:varchar(128);not null"`
	Slug      string    `json:"slug" gorm:"type:varchar(64);uniqueIndex;not null"` // 租户标识，如 "team-a"
	CreatedAt time.Time `json:"created_at"`
}

func (Tenant) TableName() string { return "tenant" }
