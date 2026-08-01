package repository

import (
	"fmt"
	"log"

	"accounter/backend/internal/config"
	"accounter/backend/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var db *gorm.DB

// NewDB 初始化数据库连接
func NewDB(cfg *config.Config) (*gorm.DB, error) {
	var dialector gorm.Dialector

	switch cfg.DB.Driver {
	case "sqlite":
		dialector = sqlite.Open(cfg.SQLite.Path)
		log.Printf("使用 SQLite 数据库: %s", cfg.SQLite.Path)
	case "mysql":
		if cfg.DB.DSN == "" {
			return nil, fmt.Errorf("MySQL DSN 未配置")
		}
		dialector = mysql.Open(cfg.DB.DSN)
		log.Printf("使用 MySQL 数据库: %s@%s/%s", cfg.DB.User, cfg.DB.Host, cfg.DB.Name)
	default:
		return nil, fmt.Errorf("不支持的数据库驱动: %s", cfg.DB.Driver)
	}

	// 日志级别：开发环境用 Warn，生产用 Silent
	logLevel := logger.Silent
	if cfg.IsDev() {
		logLevel = logger.Warn
	}

	var err error
	db, err = gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})
	if err != nil {
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}

	// AutoMigrate
	if err := db.AutoMigrate(model.AllModels()...); err != nil {
		return nil, fmt.Errorf("AutoMigrate 失败: %w", err)
	}

	// 运行迁移脚本（老数据迁移到账本体系）
	if err := migrateToLedgers(db); err != nil {
		log.Printf("警告: migrateToLedgers 失败: %v", err)
	}

	return db, nil
}

// GetDB 获取全局数据库实例
func GetDB() *gorm.DB {
	if db == nil {
		log.Fatal("数据库未初始化，请先调用 repository.NewDB()")
	}
	return db
}

// migrateToLedgers 幂等迁移：为每个没有个人账本的用户创建个人账本，并回填 ledger_id
func migrateToLedgers(db *gorm.DB) error {
	// 1. 查找所有没有个人账本的用户
	var users []model.User
	if err := db.Find(&users).Error; err != nil {
		return err
	}

	for _, user := range users {
		// 检查该用户是否已有个人账本
		var count int64
		db.Model(&model.Ledger{}).Where("owner_id = ? AND is_personal = ?", user.ID, true).Count(&count)
		if count > 0 {
			continue
		}

		// 2. 为该用户创建个人账本（必须生成 invite_code，否则空字符串触发 UNIQUE 冲突）
		ledger := model.Ledger{
			Name:       user.Nickname + "的个人账本",
			OwnerID:    user.ID,
			IsPersonal: true,
			InviteCode: model.GenInviteCode(),
		}
		if err := db.Create(&ledger).Error; err != nil {
			log.Printf("为用户 %d 创建个人账本失败: %v", user.ID, err)
			continue
		}

		// 3. 创建 owner 成员关系
		membership := model.Membership{
			LedgerID: ledger.ID,
			UserID:   user.ID,
			Role:     "owner",
		}
		if err := db.Create(&membership).Error; err != nil {
			log.Printf("为用户 %d 创建成员关系失败: %v", user.ID, err)
			continue
		}

		// 4. 幂等回填：把该用户名下 ledger_id=0 或 NULL 的数据更新为个人账本 id
		// 关键：用 Unscoped() 连软删除行一并回填
		tables := []string{"accounts", "categories", "transactions", "budgets"}
		for _, table := range tables {
			sql := fmt.Sprintf("UPDATE %s SET ledger_id = ? WHERE (ledger_id = 0 OR ledger_id IS NULL) AND user_id = ?", table)
			if err := db.Unscoped().Exec(sql, ledger.ID, user.ID).Error; err != nil {
				log.Printf("回填表 %s 失败: %v", table, err)
			}
		}

		log.Printf("为用户 %d (%s) 创建个人账本 %d 并完成数据迁移", user.ID, user.Username, ledger.ID)
	}

	return nil
}
