package config

import (
	"strings"
	"testing"

	mysqldriver "github.com/go-sql-driver/mysql"
)

func TestBuildDSNKeepsMySQLDriverDefaults(t *testing.T) {
	dsn := buildDSN(DBConfig{
		Host:     "rm-example.mysql.rds.aliyuncs.com",
		Port:     3306,
		User:     "accounter",
		Password: "p@ss:#word",
		Name:     "accounter",
	})

	parsed, err := mysqldriver.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("解析构造出的 DSN 失败: %v", err)
	}
	if !parsed.AllowNativePasswords {
		t.Fatal("MySQL DSN 必须允许 mysql_native_password 认证")
	}
	if parsed.Passwd != "p@ss:#word" {
		t.Fatalf("特殊字符密码没有被正确保留: %q", parsed.Passwd)
	}
	if parsed.Addr != "rm-example.mysql.rds.aliyuncs.com:3306" {
		t.Fatalf("RDS 地址不正确: %q", parsed.Addr)
	}
	if !parsed.ParseTime {
		t.Fatal("MySQL DSN 必须启用 parseTime")
	}
}

func TestValidateProductionRequiresActivationSecret(t *testing.T) {
	production := Config{
		DB:  DBConfig{Driver: "mysql", DSN: "configured"},
		JWT: JWTConfig{Secret: strings.Repeat("j", 32)},
		AI: AIConfig{
			Enabled:             true,
			ActivationSecret:    "CHANGE_TO_ANOTHER_RANDOM_SECRET",
			TrialDays:           7,
			TrialDailyLimit:     50,
			ActivatedDailyLimit: 500,
		},
	}
	if err := production.ValidateProduction(); err == nil {
		t.Fatal("生产配置不应接受激活码占位密钥")
	}
	production.AI.ActivationSecret = strings.Repeat("a", 32)
	if err := production.ValidateProduction(); err != nil {
		t.Fatalf("有效生产配置不应被拒绝: %v", err)
	}
}

func TestValidateProductionAllowsDisabledAIWithoutActivationConfig(t *testing.T) {
	production := Config{
		DB:  DBConfig{Driver: "mysql", DSN: "configured"},
		JWT: JWTConfig{Secret: strings.Repeat("j", 32)},
		AI:  AIConfig{Enabled: false},
	}
	if err := production.ValidateProduction(); err != nil {
		t.Fatalf("AI 关闭时不应要求 AI 激活配置: %v", err)
	}
}
