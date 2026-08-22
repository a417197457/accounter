package repository

import (
	"fmt"
	"log"
	"strings"

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

	models := model.AllModels()
	migrationDB := db
	if cfg.DB.Driver == "mysql" {
		// 不能依赖 RDS 数据库的默认字符集；分类图标和备注都可能包含 emoji。
		migrationDB = db.Set("gorm:table_options", "ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci")
	}

	// AutoMigrate
	if err := migrationDB.AutoMigrate(models...); err != nil {
		return nil, fmt.Errorf("AutoMigrate 失败: %w", err)
	}
	if cfg.DB.Driver == "mysql" {
		if err := ensureMySQLUTF8MB4(db, models); err != nil {
			return nil, fmt.Errorf("迁移 MySQL 字符集到 utf8mb4 失败: %w", err)
		}
	}
	if err := migrateLedgerTypes(db); err != nil {
		return nil, fmt.Errorf("迁移账本类型失败: %w", err)
	}

	// 运行迁移脚本（老数据迁移到账本体系）
	if err := migrateToLedgers(db); err != nil {
		log.Printf("警告: migrateToLedgers 失败: %v", err)
	}

	// 刷新默认分类图标（二级差异化、「其他」去问号）
	if err := syncCategoryIcons(db); err != nil {
		log.Printf("警告: syncCategoryIcons 失败: %v", err)
	}

	return db, nil
}

// migrateLedgerTypes 为旧账本补齐类型：个人账本保持 personal，其余默认为 family。
func migrateLedgerTypes(db *gorm.DB) error {
	migrationDB := db.Session(&gorm.Session{SkipHooks: true})
	if err := migrationDB.Model(&model.Ledger{}).
		Where("is_personal = ?", true).
		Update("type", model.LedgerTypePersonal).Error; err != nil {
		return err
	}
	return migrationDB.Model(&model.Ledger{}).
		Where("is_personal = ? AND (type IS NULL OR type = '' OR type = ?)", false, model.LedgerTypePersonal).
		Update("type", model.LedgerTypeFamily).Error
}

// ensureMySQLUTF8MB4 将旧环境中继承 utf8/utf8mb3 默认值的表转换为 utf8mb4。
// 仅在表或字符列仍不是 utf8mb4 时执行，避免每次启动都锁表。
func ensureMySQLUTF8MB4(db *gorm.DB, models []interface{}) error {
	for _, item := range models {
		stmt := &gorm.Statement{DB: db}
		if err := stmt.Parse(item); err != nil {
			return err
		}
		tableName := stmt.Schema.Table

		var tableCollation string
		if err := db.Raw(
			`SELECT COALESCE(TABLE_COLLATION, '') FROM information_schema.TABLES WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?`,
			tableName,
		).Scan(&tableCollation).Error; err != nil {
			return err
		}

		var incompatibleColumns int64
		if err := db.Raw(
			`SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND CHARACTER_SET_NAME IS NOT NULL AND CHARACTER_SET_NAME <> 'utf8mb4'`,
			tableName,
		).Scan(&incompatibleColumns).Error; err != nil {
			return err
		}

		if strings.HasPrefix(strings.ToLower(tableCollation), "utf8mb4_") && incompatibleColumns == 0 {
			continue
		}

		quotedTable := "`" + strings.ReplaceAll(tableName, "`", "``") + "`"
		if err := db.Exec(
			fmt.Sprintf("ALTER TABLE %s CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci", quotedTable),
		).Error; err != nil {
			return fmt.Errorf("转换表 %s 失败: %w", tableName, err)
		}
		log.Printf("已将 MySQL 表 %s 转换为 utf8mb4", tableName)
	}
	return nil
}

func syncCategoryIcons(db *gorm.DB) error {
	// 避免 repository → service 循环依赖：内联最小更新
	if err := db.Exec(`UPDATE categories SET icon = ? WHERE name = ? AND parent_id IS NULL AND icon = ? AND deleted_at IS NULL`,
		"📦", "其他", "❓").Error; err != nil {
		return err
	}
	childIcons := map[string]string{
		"早餐": "🥐", "午餐": "🍱", "晚餐": "🍜", "零食饮料": "🥤",
		"公交地铁": "🚇", "打车": "🚕", "加油": "⛽", "停车": "🅿️",
		"日用品": "🧴", "服装": "👕", "电子产品": "💻", "其他": "🛍️",
		"房租": "🔑", "水电燃气": "💡", "物业费": "🏢", "维修": "🔧",
		"电影": "🎬", "游戏": "🕹️", "旅游": "✈️", "运动": "⚽",
		"门诊": "🏥", "药品": "💉", "体检": "🩺",
		"书籍": "📖", "课程": "🎓", "培训": "✏️",
		"话费": "📞", "网费": "📶",
		"礼金": "🧧", "请客": "🥂", "红包": "💌",
	}
	for name, icon := range childIcons {
		if err := db.Exec(
			`UPDATE categories SET icon = ? WHERE name = ? AND parent_id IS NOT NULL AND (icon IS NULL OR icon = '' OR icon = ?) AND deleted_at IS NULL`,
			icon, name, "❓",
		).Error; err != nil {
			return err
		}
	}
	return nil
}

// GetDB 获取全局数据库实例
func GetDB() *gorm.DB {
	if db == nil {
		log.Fatal("数据库未初始化，请先调用 repository.NewDB()")
	}
	return db
}

// migrateToLedgers 幂等迁移：确保每人有个人账本，并独立回填 orphan ledger_id
func migrateToLedgers(db *gorm.DB) error {
	// 旧版本的 bank 统一迁移为 savings；信用卡仍使用 credit。
	if err := db.Model(&model.Account{}).Where("type = ?", "bank").Update("type", "savings").Error; err != nil {
		log.Printf("迁移账户类型 bank -> savings 失败: %v", err)
	}
	// 名称含「信用卡」但类型不对的账户，纠正为 credit（欠款语义）
	if err := db.Model(&model.Account{}).
		Where("name LIKE ? AND type <> ?", "%信用卡%", "credit").
		Update("type", "credit").Error; err != nil {
		log.Printf("迁移信用卡账户类型失败: %v", err)
	}

	var users []model.User
	if err := db.Find(&users).Error; err != nil {
		return err
	}

	for _, user := range users {
		var ledger model.Ledger
		err := db.Where("owner_id = ? AND is_personal = ?", user.ID, true).First(&ledger).Error
		if err != nil {
			name := user.Nickname
			if name == "" {
				name = user.Username
			}
			ledger = model.Ledger{
				Name:       name + "的个人账本",
				OwnerID:    user.ID,
				IsPersonal: true,
				Type:       model.LedgerTypePersonal,
				InviteCode: model.GenInviteCode(),
			}
			if err := db.Create(&ledger).Error; err != nil {
				log.Printf("为用户 %d 创建个人账本失败: %v", user.ID, err)
				continue
			}
			if err := db.Create(&model.Membership{
				LedgerID: ledger.ID,
				UserID:   user.ID,
				Role:     "owner",
			}).Error; err != nil {
				log.Printf("为用户 %d 创建成员关系失败: %v", user.ID, err)
				continue
			}
			log.Printf("为用户 %d (%s) 创建个人账本 %d", user.ID, user.Username, ledger.ID)
		}

		// 回填与「是否新建账本」解耦：每次启动都处理 orphan 行
		tables := []string{"accounts", "categories", "transactions", "budgets"}
		for _, table := range tables {
			sql := fmt.Sprintf(
				"UPDATE %s SET ledger_id = ? WHERE (ledger_id = 0 OR ledger_id IS NULL) AND user_id = ?",
				table,
			)
			if err := db.Unscoped().Exec(sql, ledger.ID, user.ID).Error; err != nil {
				log.Printf("回填表 %s (user=%d) 失败: %v", table, user.ID, err)
			}
		}
	}

	return nil
}
