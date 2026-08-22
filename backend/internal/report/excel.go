package report

import (
	"fmt"
	"time"

	"accounter/backend/internal/model"

	"github.com/xuri/excelize/v2"
)

// ExportTransactions 导出流水到 Excel，返回文件内容 bytes
func ExportTransactions(transactions []model.Transaction, month string) (*excelize.File, error) {
	f := excelize.NewFile()
	if err := f.SetSheetName(f.GetSheetName(0), "流水明细"); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("创建流水明细工作表: %w", err)
	}

	// 标题行
	headers := []string{"日期", "分类", "实际账户", "支付渠道", "转入账户", "操作类型", "金额", "备注", "记账人"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue("流水明细", cell, h)
	}

	// 表头样式
	style, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#F0F0F0"}, Pattern: 1},
		Alignment: &excelize.Alignment{Vertical: "center", Horizontal: "center"},
	})
	_ = f.SetCellStyle("流水明细", "A1", "I1", style)

	opTypeMap := map[string]string{
		"expense":    "支出",
		"income":     "收入",
		"borrow_in":  "借入",
		"borrow_out": "借出",
		"transfer":   "转账",
	}

	for i, tx := range transactions {
		row := i + 2
		occurred := tx.OccurredAt.Local().Format("2006-01-02 15:04")
		categoryName := ""
		if tx.Category != nil {
			categoryName = tx.Category.Name
		}
		accountName := ""
		if tx.Account != nil {
			accountName = tx.Account.Name
		}
		paymentName := ""
		if tx.PaymentAccount != nil {
			paymentName = tx.PaymentAccount.Name
		}
		toAccountName := ""
		if tx.ToAccount != nil {
			toAccountName = tx.ToAccount.Name
		}
		memberName := ""
		if tx.Member != nil {
			memberName = tx.Member.Nickname
			if memberName == "" {
				memberName = tx.Member.Username
			}
		}

		_ = f.SetCellValue("流水明细", fmt.Sprintf("A%d", row), occurred)
		_ = f.SetCellValue("流水明细", fmt.Sprintf("B%d", row), categoryName)
		_ = f.SetCellValue("流水明细", fmt.Sprintf("C%d", row), accountName)
		_ = f.SetCellValue("流水明细", fmt.Sprintf("D%d", row), paymentName)
		_ = f.SetCellValue("流水明细", fmt.Sprintf("E%d", row), toAccountName)
		_ = f.SetCellValue("流水明细", fmt.Sprintf("F%d", row), opTypeMap[tx.OpType])
		_ = f.SetCellValue("流水明细", fmt.Sprintf("G%d", row), tx.Amount.String())
		_ = f.SetCellValue("流水明细", fmt.Sprintf("H%d", row), tx.Remark)
		_ = f.SetCellValue("流水明细", fmt.Sprintf("I%d", row), memberName)
	}

	// 列宽
	widths := map[string]float64{"A": 20, "B": 12, "C": 14, "D": 12, "E": 14, "F": 10, "G": 12, "H": 24, "I": 12}
	for col, w := range widths {
		_ = f.SetColWidth("流水明细", col, col, w)
	}

	// 文件属性
	_ = f.SetDocProps(&excelize.DocProperties{
		Category: "记账",
		Title:    fmt.Sprintf("Accounter %s 流水明细", month),
		Created:  time.Now().Format("2006-01-02 15:04:05"),
	})

	return f, nil
}
