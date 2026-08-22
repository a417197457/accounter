package config

import (
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/spf13/viper"
)

// Config 全局配置结构
type Config struct {
	Server  ServerConfig
	DB      DBConfig
	SQLite  SQLiteConfig
	JWT     JWTConfig
	AI      AIConfig
	WeChat  WeChatConfig
	Logging LoggingConfig
	Storage StorageConfig
}

// StorageConfig controls persistent user-uploaded files.
type StorageConfig struct {
	UploadDir string
}

// LoggingConfig controls the production rotating application log.
type LoggingConfig struct {
	File       string
	MaxSizeMB  int
	MaxBackups int
	MaxAgeDays int
	Compress   bool
}

// WeChatConfig 微信小程序服务端配置。Secret 只能通过服务端配置或环境变量注入。
type WeChatConfig struct {
	AppID     string
	AppSecret string
	APIBase   string
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Port           int
	AllowedOrigins []string
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
	BaseURL                  string
	APIKey                   string
	Model                    string
	ASR                      ASRConfig
	ActivationSecret         string
	TrialDays                int
	TrialDailyLimit          int
	ActivatedDailyLimit      int
	ActivationAttemptLimit   int
	ActivationIPAttemptLimit int
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
	v.SetDefault("server.allowed_origins", []string{})
	v.SetDefault("db.driver", "sqlite")
	v.SetDefault("jwt.secret", "dev-secret")
	v.SetDefault("jwt.expire", "168h")
	v.SetDefault("ai.base_url", "https://dashscope.aliyuncs.com/compatible-mode/v1")
	v.SetDefault("ai.model", "qwen-max")
	v.SetDefault("ai.asr.model", "qwen3-asr-flash")
	v.SetDefault("ai.activation_secret", "dev-activation-secret")
	v.SetDefault("ai.trial_days", 7)
	v.SetDefault("ai.trial_daily_limit", 50)
	v.SetDefault("ai.activated_daily_limit", 500)
	v.SetDefault("ai.activation_attempt_limit", 5)
	v.SetDefault("ai.activation_ip_attempt_limit", 30)
	v.SetDefault("wechat.api_base", "https://api.weixin.qq.com")
	v.SetDefault("logging.max_size_mb", 20)
	v.SetDefault("logging.max_backups", 14)
	v.SetDefault("logging.max_age_days", 30)
	v.SetDefault("logging.compress", true)
	v.SetDefault("storage.upload_dir", "./uploads")

	// 读取配置文件
	if err := v.ReadInConfig(); err != nil {
		log.Printf("警告: 读取配置文件失败: %v，使用默认配置", err)
	}

	// 应用环境变量覆盖
	applyEnvOverrides(v)

	// 解析配置
	cfg = &Config{
		Server: ServerConfig{
			Port:           v.GetInt("server.port"),
			AllowedOrigins: normalizeStringSlice(v.GetStringSlice("server.allowed_origins")),
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
			BaseURL:                  v.GetString("ai.base_url"),
			APIKey:                   v.GetString("ai.api_key"),
			Model:                    v.GetString("ai.model"),
			ActivationSecret:         v.GetString("ai.activation_secret"),
			TrialDays:                v.GetInt("ai.trial_days"),
			TrialDailyLimit:          v.GetInt("ai.trial_daily_limit"),
			ActivatedDailyLimit:      v.GetInt("ai.activated_daily_limit"),
			ActivationAttemptLimit:   v.GetInt("ai.activation_attempt_limit"),
			ActivationIPAttemptLimit: v.GetInt("ai.activation_ip_attempt_limit"),
			ASR: ASRConfig{
				BaseURL: v.GetString("ai.asr.base_url"),
				APIKey:  v.GetString("ai.asr.api_key"),
				Model:   v.GetString("ai.asr.model"),
			},
		},
		WeChat: WeChatConfig{
			AppID:     v.GetString("wechat.app_id"),
			AppSecret: v.GetString("wechat.app_secret"),
			APIBase:   v.GetString("wechat.api_base"),
		},
		Logging: LoggingConfig{
			File:       v.GetString("logging.file"),
			MaxSizeMB:  v.GetInt("logging.max_size_mb"),
			MaxBackups: v.GetInt("logging.max_backups"),
			MaxAgeDays: v.GetInt("logging.max_age_days"),
			Compress:   v.GetBool("logging.compress"),
		},
		Storage: StorageConfig{
			UploadDir: v.GetString("storage.upload_dir"),
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
		"SERVER_PORT":                    "server.port",
		"CORS_ALLOWED_ORIGINS":           "server.allowed_origins",
		"DB_DRIVER":                      "db.driver",
		"DB_DSN":                         "db.dsn",
		"DB_HOST":                        "db.host",
		"DB_PORT":                        "db.port",
		"DB_USER":                        "db.user",
		"DB_PASSWORD":                    "db.password",
		"DB_NAME":                        "db.name",
		"SQLITE_PATH":                    "sqlite.path",
		"JWT_SECRET":                     "jwt.secret",
		"JWT_EXPIRE":                     "jwt.expire",
		"AI_BASE_URL":                    "ai.base_url",
		"DASHSCOPE_API_KEY":              "ai.api_key",
		"AI_MODEL":                       "ai.model",
		"AI_ASR_BASE_URL":                "ai.asr.base_url",
		"AI_ASR_API_KEY":                 "ai.asr.api_key",
		"AI_ASR_MODEL":                   "ai.asr.model",
		"AI_ACTIVATION_SECRET":           "ai.activation_secret",
		"AI_TRIAL_DAYS":                  "ai.trial_days",
		"AI_TRIAL_DAILY_LIMIT":           "ai.trial_daily_limit",
		"AI_ACTIVATED_DAILY_LIMIT":       "ai.activated_daily_limit",
		"AI_ACTIVATION_ATTEMPT_LIMIT":    "ai.activation_attempt_limit",
		"AI_ACTIVATION_IP_ATTEMPT_LIMIT": "ai.activation_ip_attempt_limit",
		"WECHAT_APP_ID":                  "wechat.app_id",
		"WECHAT_APP_SECRET":              "wechat.app_secret",
		"WECHAT_API_BASE":                "wechat.api_base",
		"LOG_FILE":                       "logging.file",
		"LOG_MAX_SIZE_MB":                "logging.max_size_mb",
		"LOG_MAX_BACKUPS":                "logging.max_backups",
		"LOG_MAX_AGE_DAYS":               "logging.max_age_days",
		"LOG_COMPRESS":                   "logging.compress",
		"UPLOAD_DIR":                     "storage.upload_dir",
	}

	for envKey, viperKey := range envMap {
		if val := os.Getenv(envKey); val != "" {
			v.Set(viperKey, val)
		}
	}
}

// buildDSN 拼接 MySQL DSN
func buildDSN(db DBConfig) string {
	// 必须从 NewConfig 开始，保留驱动的安全默认值；直接创建 Config
	// 会让 AllowNativePasswords 等默认开启项变成 false，导致 RDS 账号
	// 使用 mysql_native_password 时认证失败。
	dsnConfig := mysqldriver.NewConfig()
	dsnConfig.User = db.User
	dsnConfig.Passwd = db.Password
	dsnConfig.Net = "tcp"
	dsnConfig.Addr = net.JoinHostPort(db.Host, fmt.Sprint(db.Port))
	dsnConfig.DBName = db.Name
	dsnConfig.ParseTime = true
	dsnConfig.Loc = time.Local
	dsnConfig.Params = map[string]string{"charset": "utf8mb4"}
	return dsnConfig.FormatDSN()
}

func normalizeStringSlice(values []string) []string {
	var result []string
	for _, value := range values {
		for _, item := range strings.Split(value, ",") {
			if item = strings.TrimSpace(item); item != "" {
				result = append(result, strings.TrimRight(item, "/"))
			}
		}
	}
	return result
}

// ValidateProduction 拒绝携带开发密钥启动 MySQL 生产模式。
func (c *Config) ValidateProduction() error {
	if c.DB.Driver != "mysql" {
		return nil
	}
	if c.DB.DSN == "" {
		return fmt.Errorf("生产环境缺少 MySQL 连接配置")
	}
	if len(c.JWT.Secret) < 32 || c.JWT.Secret == "dev-secret" || c.JWT.Secret == "change-me-in-production" {
		return fmt.Errorf("生产环境 JWT_SECRET 必须是至少 32 位的随机字符串")
	}
	if len(c.AI.ActivationSecret) < 32 || c.AI.ActivationSecret == "dev-activation-secret" || strings.HasPrefix(c.AI.ActivationSecret, "CHANGE_") {
		return fmt.Errorf("生产环境 AI_ACTIVATION_SECRET 必须是至少 32 位的随机字符串")
	}
	if c.AI.TrialDays <= 0 || c.AI.TrialDailyLimit <= 0 || c.AI.ActivatedDailyLimit <= 0 {
		return fmt.Errorf("AI 试用天数和每日调用上限必须大于 0")
	}
	return nil
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
