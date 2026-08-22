package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"text/tabwriter"
	"time"
	"unicode/utf8"

	"accounter/backend/internal/service"
)

func runOpsUsers(svc *service.OperationsService, args []string) {
	flags := flag.NewFlagSet("ops users", flag.ExitOnError)
	page := flags.Int("page", 1, "页码，从 1 开始")
	pageSize := flags.Int("page-size", 50, "每页数量，最大 200")
	_ = flags.Parse(args)

	records, total, err := svc.ListUserSummaries(*page, *pageSize)
	if err != nil {
		log.Fatalf("查询用户汇总失败: %v", err)
	}
	fmt.Printf("注册用户总数: %d；当前第 %d 页，每页 %d 条\n", total, *page, *pageSize)
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "用户ID\t昵称\t登录方式\t登录用户名\t资金账户\t流水数\t支出合计\t收入合计\t最近流水\t注册时间")
	for _, record := range records {
		username := record.Username
		if record.LoginType == "wechat" {
			username = "-"
		}
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%d\t%d\t%s\t%s\t%s\t%s\n",
			record.UserID,
			safeCell(displayName(record.Nickname, record.Username)),
			loginTypeLabel(record.LoginType),
			safeCell(username),
			record.AccountCount,
			record.BillCount,
			record.ExpenseTotal.StringFixed(2),
			record.IncomeTotal.StringFixed(2),
			formatOptionalTime(record.LastOccurredAt),
			record.CreatedAt.In(time.Local).Format("2006-01-02 15:04"),
		)
	}
	_ = w.Flush()
}

func runOpsBills(svc *service.OperationsService, args []string) {
	flags := flag.NewFlagSet("ops bills", flag.ExitOnError)
	userID := flags.Uint("user-id", 0, "按用户 ID 过滤；0 表示全部用户")
	month := flags.String("month", "", "按月份过滤，格式 YYYY-MM")
	page := flags.Int("page", 1, "页码，从 1 开始")
	pageSize := flags.Int("page-size", 50, "每页数量，最大 200")
	showRemark := flags.Bool("show-remark", false, "显示账单备注；默认隐藏敏感备注")
	_ = flags.Parse(args)

	records, total, err := svc.ListBills(service.OpsBillFilter{
		UserID: *userID, Month: *month, Page: *page, PageSize: *pageSize,
	})
	if err != nil {
		log.Fatalf("查询账单失败: %v", err)
	}
	filterText := "全部用户"
	if *userID > 0 {
		filterText = "用户 ID " + strconv.FormatUint(uint64(*userID), 10)
	}
	if *month != "" {
		filterText += "，月份 " + *month
	}
	fmt.Printf("账单记录总数: %d（%s）；当前第 %d 页，每页 %d 条\n", total, filterText, *page, *pageSize)
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "流水ID\t用户ID\t用户\t发生时间\t类型\t金额\t账户\t分类\t备注")
	for _, record := range records {
		remark := "[已隐藏，使用 --show-remark 查看]"
		if *showRemark {
			remark = truncateCell(record.Remark, 60)
			if remark == "" {
				remark = "-"
			}
		}
		fmt.Fprintf(w, "%d\t%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			record.ID,
			record.OwnerUserID,
			safeCell(displayName(record.OwnerNickname, record.OwnerUsername)),
			record.OccurredAt.In(time.Local).Format("2006-01-02 15:04"),
			opTypeLabel(record.OpType),
			record.Amount.StringFixed(2),
			safeCell(record.AccountName),
			safeCell(record.CategoryName),
			remark,
		)
	}
	_ = w.Flush()
}

func displayName(nickname, username string) string {
	if nickname != "" {
		return nickname
	}
	return username
}

func loginTypeLabel(value string) string {
	if value == "wechat" {
		return "微信"
	}
	return "密码"
}

func opTypeLabel(value string) string {
	label := map[string]string{
		"expense": "支出", "income": "收入", "borrow_in": "借入", "borrow_out": "借出", "transfer": "转账",
	}[value]
	if label == "" {
		return value
	}
	return label
}

func formatOptionalTime(value *time.Time) string {
	if value == nil {
		return "-"
	}
	return value.In(time.Local).Format("2006-01-02 15:04")
}

func truncateCell(value string, maxRunes int) string {
	value = safeCell(value)
	if utf8.RuneCountInString(value) <= maxRunes {
		return value
	}
	runes := []rune(value)
	return string(runes[:maxRunes]) + "…"
}
