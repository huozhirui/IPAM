// Package model 定义数据库表对应的 GORM 模型。
package model

import "time"

// IPPool 网段池，支持两种定义方式：
//   - CIDR 模式：如 10.0.0.0/16
//   - 范围模式：如 10.0.0.0 - 10.0.255.255
//
// 不论哪种输入方式，都会同时存储 CIDR（或显示用字符串）和 StartIP/EndIP。
type IPPool struct {
	ID          uint64    `json:"id" gorm:"primaryKey;autoIncrement"`
	CIDR        string    `json:"cidr" gorm:"type:varchar(100);not null"`               // CIDR 或 "startIP - endIP" 显示串
	StartIP     string    `json:"start_ip" gorm:"type:varchar(43);not null;default:''"` // 范围起始 IP
	EndIP       string    `json:"end_ip" gorm:"type:varchar(43);not null;default:''"`   // 范围结束 IP
	Name        string    `json:"name" gorm:"type:varchar(128);not null;default:''"`    // 网段池名称
	Description string    `json:"description" gorm:"type:varchar(512);default:''"`      // 备注说明
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (IPPool) TableName() string { return "ip_pool" }
