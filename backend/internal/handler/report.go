package handler

import (
	"fmt"
	"net/http"
	"time"

	"accounter/backend/internal/middleware"
	"accounter/backend/internal/model"
	"accounter/backend/internal/report"
	"accounter/backend/internal/repository"

	"github.com/gin-gonic/gin"
)

// ReportHandler 报表处理器
type ReportHandler struct{}

// NewReportHandler 创建报表处理器
func NewReportHandler() *ReportHandler {
	return &ReportHandler{}
}

// Monthly 月度报表（返回该月流水）
func (h *ReportHandler) Monthly(c *gin.Context) {
	ledgerID := middleware.LedgerID(c)
	monthStr := c.Query("month")
	if monthStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 month 参数"})
		return
	}

	// 解析月份 YYYY-MM
	t, err := time.ParseInLocation("2006-01", monthStr, time.Local)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "month 格式错误，应为 YYYY-MM"})
		return
	}
	start := t
	end := t.AddDate(0, 1, 0)

	var txs []model.Transaction
	err = repository.GetDB().
		Where("ledger_id = ? AND occurred_at >= ? AND occurred_at < ? AND deleted_at IS NULL", ledgerID, start, end).
		Preload("Account").Preload("PaymentAccount").Preload("ToAccount").Preload("Category").Preload("Category.Parent").Preload("Member").
		Order("occurred_at desc, id desc").
		Find(&txs).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if txs == nil {
		txs = []model.Transaction{}
	}
	c.JSON(http.StatusOK, gin.H{"data": txs})
}

// Export 导出 Excel
func (h *ReportHandler) Export(c *gin.Context) {
	ledgerID := middleware.LedgerID(c)
	monthStr := c.Query("month")

	query := repository.GetDB().Model(&model.Transaction{}).
		Where("ledger_id = ? AND deleted_at IS NULL", ledgerID)

	if monthStr != "" {
		t, err := time.ParseInLocation("2006-01", monthStr, time.Local)
		if err == nil {
			query = query.Where("occurred_at >= ? AND occurred_at < ?", t, t.AddDate(0, 1, 0))
		}
	}
	// 可选：账户/分类筛选
	if s := c.Query("account_id"); s != "" {
		query = query.Where("account_id = ?", s)
	}

	var txs []model.Transaction
	if err := query.Preload("Account").Preload("PaymentAccount").Preload("ToAccount").Preload("Category").Preload("Category.Parent").Preload("Member").
		Order("occurred_at desc, id desc").
		Find(&txs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	f, err := report.ExportTransactions(txs, monthStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成 Excel 失败: " + err.Error()})
		return
	}

	filename := "accounter.xlsx"
	if monthStr != "" {
		filename = fmt.Sprintf("accounter_%s.xlsx", monthStr)
	}
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))

	if err := f.Write(c.Writer); err != nil {
		_ = f.Close()
		return
	}
	_ = f.Close()
}
