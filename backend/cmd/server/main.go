package main

import (
	"fmt"
	"log"
	"os"

	"accounter/backend/internal/ai"
	"accounter/backend/internal/config"
	"accounter/backend/internal/handler"
	"accounter/backend/internal/middleware"
	"accounter/backend/internal/observability"
	"accounter/backend/internal/repository"
	"accounter/backend/internal/service"

	"github.com/gin-gonic/gin"
)

func main() {
	// 加载配置
	cfg := config.Load()
	if err := cfg.ValidateProduction(); err != nil {
		log.Fatalf("生产配置校验失败: %v", err)
	}
	logFile := cfg.Logging.File
	if logFile == "" && !cfg.IsDev() {
		logFile = "/opt/accounter/logs/accounter.log"
	}
	logCloser, err := observability.Setup(logFile, cfg.Logging.MaxSizeMB, cfg.Logging.MaxBackups, cfg.Logging.MaxAgeDays, cfg.Logging.Compress)
	if err != nil {
		log.Fatalf("日志初始化失败: %v", err)
	}
	if logCloser != nil {
		defer logCloser.Close()
	}

	// 初始化数据库
	if _, err := repository.NewDB(cfg); err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
	}
	if repaired, err := service.NewTransactionService().RepairSyncedPaymentRoutes(); err != nil {
		log.Printf("警告: 修复存量同步流水支付方式失败: %v", err)
	} else if repaired > 0 {
		log.Printf("已修复 %d 条同步流水的支付方式", repaired)
	}

	// 设置 Gin 模式
	if cfg.IsDev() {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	// 创建 Gin 引擎
	r := gin.Default()
	r.Use(observability.AuditRequests())
	// 仅信任同机 Nginx，避免公网请求伪造 X-Forwarded-For，同时让激活限流取得真实来源 IP。
	if err := r.SetTrustedProxies([]string{"127.0.0.1", "::1"}); err != nil {
		log.Fatalf("代理配置失败: %v", err)
	}

	// CORS 中间件
	r.Use(middleware.CORS(cfg.Server.AllowedOrigins, cfg.IsDev()))
	if err := os.MkdirAll(cfg.Storage.UploadDir, 0750); err != nil {
		log.Fatalf("创建上传目录失败: %v", err)
	}
	r.StaticFS("/api/uploads", gin.Dir(cfg.Storage.UploadDir, false))

	// 健康检查端点
	r.GET("/health", func(c *gin.Context) {
		sqlDB, err := repository.GetDB().DB()
		if err != nil || sqlDB.PingContext(c.Request.Context()) != nil {
			c.JSON(503, gin.H{"status": "error", "database": false})
			return
		}
		c.JSON(200, gin.H{
			"status":   "ok",
			"database": true,
			"ai":       cfg.AI.APIKey != "" && cfg.AI.APIKey != "YOUR_BAILIAN_API_KEY",
		})
	})

	// 初始化 handlers
	authHandler := handler.NewAuthHandler()
	ledgerHandler := handler.NewLedgerHandler()
	accountHandler := handler.NewAccountHandler()
	categoryHandler := handler.NewCategoryHandler()
	transactionHandler := handler.NewTransactionHandler()
	statsHandler := handler.NewStatsHandler()
	budgetHandler := handler.NewBudgetHandler()
	reportHandler := handler.NewReportHandler()
	bookkeepingPreferenceHandler := handler.NewBookkeepingPreferenceHandler()
	aiService := ai.NewService()
	aiHandler := handler.NewAIHandler(aiService)

	// 公开路由（无需登录）
	auth := r.Group("/api/auth")
	{
		auth.POST("/register", authHandler.Register)
		auth.POST("/login", authHandler.Login)
		auth.POST("/wechat", authHandler.WechatLogin)
		auth.POST("/wechat-phone", authHandler.WechatPhoneLogin)
	}

	// 需要登录的路由
	api := r.Group("/api")
	api.Use(middleware.Auth(cfg.JWT.Secret))
	{
		// 当前用户
		api.GET("/me", authHandler.GetCurrentUser)
		api.PUT("/me", authHandler.UpdateProfile)
		api.POST("/me/avatar", authHandler.UploadAvatar)

		// 账本管理（不绑定具体账本）
		ledgers := api.Group("/ledgers")
		{
			ledgers.GET("", ledgerHandler.List)
			ledgers.POST("", ledgerHandler.Create)
			ledgers.POST("/join", ledgerHandler.Join)
			ledgers.DELETE("/:id/transactions", transactionHandler.ClearSharedLedger)
			ledgers.DELETE("/:id", ledgerHandler.Delete)
			ledgers.GET("/:id/members", ledgerHandler.Members)
			ledgers.DELETE("/:id/members/:uid", ledgerHandler.RemoveMember)
			ledgers.POST("/:id/leave", ledgerHandler.Leave)
		}

		// AI 权益属于用户，不依赖当前账本。
		api.GET("/ai/entitlement", aiHandler.Entitlement)
		api.POST("/ai/activate", aiHandler.Activate)

		// 需要账本上下文的路由
		business := api.Group("")
		business.Use(middleware.RequireLedger())
		{
			// 账户
			accounts := business.Group("/accounts")
			{
				accounts.GET("", accountHandler.List)
				accounts.POST("", accountHandler.Create)
				accounts.PUT("/:id", accountHandler.Update)
				accounts.DELETE("/:id", accountHandler.Delete)
			}

			// 分类
			categories := business.Group("/categories")
			{
				categories.GET("/tree", categoryHandler.ListTree)
				categories.GET("/flat", categoryHandler.ListFlat)
				categories.POST("", categoryHandler.Create)
				categories.PUT("/:id", categoryHandler.Update)
				categories.DELETE("/:id", categoryHandler.Delete)
			}

			// 整月推送（独立路径，避免与 /transactions/:id 冲突）
			business.POST("/push-month", transactionHandler.PushMonth)
			business.POST("/batch-delete-transactions", transactionHandler.DeleteBatch)
			business.POST("/batch-push-transactions", transactionHandler.PushBatch)
			// 月份摘要同样使用独立路径，避免静态 /months 与 /:id 冲突。
			business.GET("/transaction-months", transactionHandler.ListMonths)

			// 流水
			transactions := business.Group("/transactions")
			{
				transactions.GET("", transactionHandler.List)
				transactions.GET("/:id", transactionHandler.Get)
				transactions.POST("", transactionHandler.Create)
				transactions.PUT("/:id", transactionHandler.Update)
				transactions.DELETE("/:id", transactionHandler.Delete)
				transactions.POST("/:id/push", transactionHandler.Push)
			}

			// 统计
			stats := business.Group("/stats")
			{
				stats.GET("/overview", statsHandler.Overview)
				stats.GET("/trend", statsHandler.Trend)
				stats.GET("/category", statsHandler.Category)
				stats.GET("/monthly", statsHandler.Monthly)
			}

			// 预算
			budgets := business.Group("/budgets")
			{
				budgets.GET("", budgetHandler.List)
				budgets.POST("", budgetHandler.Set)
				budgets.PUT("", budgetHandler.Set)
				budgets.DELETE("/:id", budgetHandler.Delete)
			}

			// 报表
			reports := business.Group("/report")
			{
				reports.GET("/monthly", reportHandler.Monthly)
				reports.GET("/export", reportHandler.Export)
			}

			// AI
			aiGroup := business.Group("/ai")
			{
				aiGroup.GET("/account-preference", bookkeepingPreferenceHandler.Get)
				aiGroup.PUT("/account-preference", bookkeepingPreferenceHandler.Update)
				aiGroup.POST("/chat", aiHandler.Chat)
				aiGroup.POST("/transcribe", aiHandler.Transcribe)
				aiGroup.POST("/confirm", aiHandler.Confirm)
				aiGroup.GET("/settings", aiHandler.Settings)
			}
		}
	}

	// 启动服务器
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	log.Printf("服务器启动在 %s", addr)
	log.Printf("环境: %s", map[bool]string{true: "开发", false: "生产"}[cfg.IsDev()])
	log.Printf("数据库: %s", cfg.DB.Driver)
	if cfg.AI.APIKey == "" || cfg.AI.APIKey == "YOUR_BAILIAN_API_KEY" {
		log.Printf("警告: AI API Key 未配置，AI 功能不可用")
	}

	if err := r.Run(addr); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
		os.Exit(1)
	}
}
