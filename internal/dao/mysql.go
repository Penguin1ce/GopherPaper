// Package dao 是数据访问层，封装 MySQL、Redis 等存储客户端。
package dao

import (
	"fmt"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"GopherCPP/internal/config"
)

// DB 是全局 MySQL 句柄，由 InitMySQL 初始化。
var DB *gorm.DB

// InitMySQL 用 GORM 连接 MySQL 并配置连接池，结果存入包级 DB。
func InitMySQL(cfg config.MySQLConfig) error {
	db, err := gorm.Open(mysql.Open(cfg.DSN), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("dao: 连接 MySQL 失败: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("dao: 获取 sql.DB 失败: %w", err)
	}
	if cfg.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime > 0 {
		sqlDB.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetime) * time.Second)
	}
	DB = db
	return nil
}
