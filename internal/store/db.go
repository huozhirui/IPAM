// Package store 提供数据库连接初始化与各表的 CRUD 操作。
package store

import (
	"fmt"
	"log"
	"network-plan/internal/model"

	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// InitDB 初始化数据库连接并自动迁移表结构。
//   - dsn 非空时连接 MySQL，格式: user:pass@tcp(host:port)/dbname?charset=utf8mb4&parseTime=True
//   - dsn 为空时自动使用本地 SQLite 文件 (ipam.db)，零配置开箱即用
func InitDB(dsn string) (*gorm.DB, error) {
	var db *gorm.DB
	var err error

	if dsn != "" {
		// MySQL 模式
		log.Println("Using MySQL database")
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	} else {
		// SQLite fallback 模式
		log.Println("No DSN configured, using local SQLite database: ipam.db")
		db, err = gorm.Open(sqlite.Open("ipam.db"), &gorm.Config{})
	}
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// 自动建表 / 迁移字段变更
	if err := db.AutoMigrate(
		&model.IPPool{},
		&model.Allocation{},
		&model.AuditLog{},
		&model.User{},
	); err != nil {
		return nil, fmt.Errorf("failed to auto-migrate: %w", err)
	}

	return db, nil
}
