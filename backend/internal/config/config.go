package config

import (
	"log"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config 全局配置结构
type Config struct {
	Server ServerConfig
	DB     DBConfig
	SQLite SQLiteConfig
	JWT    JWTConfig
	AI     AIConfig
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Port int
}

// DBConfig 数据库配置
type DBConfig struct {
	Driver   string // sqlite 或 mysql
	DSN      string // MySQL 连接串
	Host     string
	Port     int
	User     string
	Password string
	Name     string
}

// SQLiteConfig SQLite 配置
type SQLiteConfig struct {
	Path string
}

// JWTConfig JWT 配置
type JWTConfig struct {
	Secret string
	Expire time.Duration
}

// AIConfig AI 配置（对话和语音分离）
type AIConfig struct {
	BaseURL string
	APIKey  string
	Model   string
	ASR     ASRConfig
}

// ASRConfig 语音识别配置
type ASRConfig struct {
	BaseURL string
	APIKey  string
	Model   string
}

var cfg *Config

// Load 加载配置
func Load() *Config {
	if cfg != nil {
		return cfg
	}

	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath("./configs/")
	v.AddConfigPath("../configs/")
	v.AddConfigPath("../../configs/")

	// 默认值
	v.SetDefault("server.port", 8080)
	v.SetDefault("db.driver", "sqlite")
	v.SetDefault("jwt.secret", "dev-secret")
	v.SetDefault("jwt.expire", "168h")
	v.SetDefault("ai.base_url", "https://dashscope.aliyuncs.com/compatible-mode/v1")
	v.SetDefault("ai.model", "qwen-max")
	v.SetDefault("ai.asr.model", "qwen3-asr-flash")

	// 读取配置文件
	if err := v.ReadInConfig(); err != nil {
		log.Printf("警告: 读取配置文件失败: %v，使用默认配置", err)
	}

	// 应用环境变量覆盖
	applyEnvOverrides(v)

	// 解析配置
	cfg = &Config{
		Server: ServerConfig{
			Port: v.GetInt("server.port"),
		},
		DB: DBConfig{
			Driver:   v.GetString("db.driver"),
			DSN:      v.GetString("db.dsn"),
			Host:     v.GetString("db.host"),
			Port:     v.GetInt("db.port"),
			User:     v.GetString("db.user"),
			Password: v.GetString("db.password"),
			Name:     v.GetString("db.name"),
		},
		SQLite: SQLiteConfig{
			Path: v.GetString("sqlite.path"),
		},
		JWT: JWTConfig{
			Secret: v.GetString("jwt.secret"),
			Expire: v.GetDuration("jwt.expire"),
		},
		AI: AIConfig{
			BaseURL: v.GetString("ai.base_url"),
			APIKey:  v.GetString("ai.api_key"),
			Model:   v.GetString("ai.model"),
			ASR: ASRConfig{
				BaseURL: v.GetString("ai.asr.base_url"),
				APIKey:  v.GetString("ai.asr.api_key"),
				Model:   v.GetString("ai.asr.model"),
			},
		},
	}

	// ASR 配置回落：如果 ASR 的 base_url 或 api_key 为空，则使用对话模型的配置
	if cfg.AI.ASR.BaseURL == "" {
		cfg.AI.ASR.BaseURL = cfg.AI.BaseURL
	}
	if cfg.AI.ASR.APIKey == "" {
		cfg.AI.ASR.APIKey = cfg.AI.APIKey
	}

	// 如果 DB DSN 为空（MySQL 模式），则从各字段拼接
	if cfg.DB.Driver == "mysql" && cfg.DB.DSN == "" {
		cfg.DB.DSN = buildDSN(cfg.DB)
	}

	return cfg
}

// applyEnvOverrides 环境变量覆盖
func applyEnvOverrides(v *viper.Viper) {
	envMap := map[string]string{
		"SERVER_PORT":      "server.port",
		"DB_DRIVER":        "db.driver",
		"DB_DSN":           "db.dsn",
		"DB_HOST":          "db.host",
		"DB_PORT":          "db.port",
		"DB_USER":          "db.user",
		"DB_PASSWORD":      "db.password",
		"DB_NAME":          "db.name",
		"SQLITE_PATH":      "sqlite.path",
		"JWT_SECRET":       "jwt.secret",
		"JWT_EXPIRE":       "jwt.expire",
		"AI_BASE_URL":      "ai.base_url",
		"DASHSCOPE_API_KEY": "ai.api_key",
		"AI_MODEL":         "ai.model",
		"AI_ASR_BASE_URL":  "ai.asr.base_url",
		"AI_ASR_API_KEY":   "ai.asr.api_key",
		"AI_ASR_MODEL":     "ai.asr.model",
	}

	for envKey, viperKey := range envMap {
		if val := os.Getenv(envKey); val != "" {
			v.Set(viperKey, val)
		}
	}
}

// buildDSN 拼接 MySQL DSN
func buildDSN(db DBConfig) string {
	return db.User + ":" + db.Password + "@tcp(" + db.Host + ":" + string(rune(db.Port)) + ")/" + db.Name + "?charset=utf8mb4&parseTime=True&loc=Local"
}

// Get 获取全局配置（必须先调用 Load）
func Get() *Config {
	if cfg == nil {
		log.Fatal("配置未加载，请先调用 config.Load()")
	}
	return cfg
}

// GetDSN 获取数据库连接串
func (c *Config) GetDSN() string {
	if c.DB.Driver == "mysql" {
		return c.DB.DSN
	}
	return c.SQLite.Path
}

// IsDev 是否开发环境
func (c *Config) IsDev() bool {
	return strings.ToLower(c.DB.Driver) == "sqlite"
}
