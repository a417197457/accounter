package main

import (
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"accounter/backend/internal/config"
	"accounter/backend/internal/repository"
	"accounter/backend/internal/service"

	"github.com/subosito/gotenv"
)

func main() {
	loadEnvironmentFile()
	cfg := config.Load()
	if err := cfg.ValidateProduction(); err != nil {
		log.Fatalf("生产配置校验失败: %v", err)
	}
	args := os.Args[1:]
	if len(args) < 2 {
		printUsage()
		os.Exit(2)
	}

	switch args[0] {
	case "activation-code":
		if _, err := repository.NewDB(cfg); err != nil {
			log.Fatalf("数据库初始化失败: %v", err)
		}
		svc := service.NewAIEntitlementService()
		switch args[1] {
		case "create":
			runCreate(svc, args[2:])
		case "list":
			runList(svc, args[2:])
		case "revoke":
			runRevoke(svc, args[2:])
		default:
			printUsage()
			os.Exit(2)
		}
	case "ops":
		if _, err := repository.NewReadOnlyDB(cfg); err != nil {
			log.Fatalf("数据库连接失败: %v", err)
		}
		svc := service.NewOperationsService()
		switch args[1] {
		case "users":
			runOpsUsers(svc, args[2:])
		case "bills":
			runOpsBills(svc, args[2:])
		default:
			printUsage()
			os.Exit(2)
		}
	default:
		printUsage()
		os.Exit(2)
	}
}

func loadEnvironmentFile() {
	envFile := os.Getenv("ACCOUNTER_ENV_FILE")
	if envFile == "" {
		envFile = "/etc/accounter/env"
	}
	if _, err := os.Stat(envFile); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return
		}
		log.Fatalf("检查环境文件失败: %v", err)
	}
	if err := gotenv.Load(envFile); err != nil {
		log.Fatalf("读取环境文件失败: %v", err)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `轻账运维管理：
  accounter-admin ops users --page 1 --page-size 50
  accounter-admin ops bills --user-id 12 --month 2026-08 --page 1 --page-size 50
  accounter-admin ops bills --user-id 12 --show-remark

激活码管理：
  accounter-admin activation-code create --count 20 --batch first-release --expires 2027-01-01 --output codes.csv
  accounter-admin activation-code list --status unused --batch first-release
  accounter-admin activation-code revoke --id 123

环境文件默认读取 /etc/accounter/env，可用 ACCOUNTER_ENV_FILE 覆盖。`)
}

func parseExpiry(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	date, err := time.ParseInLocation("2006-01-02", value, time.Local)
	if err != nil {
		return nil, fmt.Errorf("到期日期格式必须为 YYYY-MM-DD")
	}
	// 指定日期当天仍可兑换，下一日零点到期。
	expiresAt := date.Add(24 * time.Hour).UTC()
	return &expiresAt, nil
}

func runCreate(svc *service.AIEntitlementService, args []string) {
	flags := flag.NewFlagSet("activation-code create", flag.ExitOnError)
	count := flags.Int("count", 1, "生成数量，1-1000")
	batch := flags.String("batch", "", "批次名称")
	expires := flags.String("expires", "", "兑换截止日期 YYYY-MM-DD，当天有效")
	assignedTo := flags.String("assigned-to", "", "发放对象备注")
	note := flags.String("note", "", "内部备注")
	output := flags.String("output", "-", "明文 CSV 输出路径；- 表示标准输出")
	_ = flags.Parse(args)

	expiresAt, err := parseExpiry(*expires)
	if err != nil {
		log.Fatal(err)
	}
	codes, err := svc.CreateActivationCodes(service.ActivationCodeCreateOptions{
		Count: *count, BatchName: *batch, ExpiresAt: expiresAt, AssignedTo: *assignedTo, Note: *note,
	})
	if err != nil {
		log.Fatalf("生成激活码失败: %v", err)
	}

	writer, closer, err := activationCSVWriter(*output)
	if err != nil {
		log.Fatal(err)
	}
	if closer != nil {
		defer closer.Close()
	}
	csvWriter := csv.NewWriter(writer)
	_ = csvWriter.Write([]string{"id", "code", "batch", "expires_at"})
	for _, code := range codes {
		expiresText := ""
		if code.ExpiresAt != nil {
			expiresText = code.ExpiresAt.In(time.Local).Format(time.RFC3339)
		}
		_ = csvWriter.Write([]string{strconv.FormatUint(uint64(code.ID), 10), code.Code, code.BatchName, expiresText})
	}
	csvWriter.Flush()
	if err := csvWriter.Error(); err != nil {
		log.Fatalf("写入 CSV 失败: %v", err)
	}
	if *output != "-" {
		fmt.Fprintf(os.Stderr, "已生成 %d 个激活码：%s（明文仅此一次，请妥善保存）\n", len(codes), *output)
	}
}

func activationCSVWriter(output string) (io.Writer, io.Closer, error) {
	if output == "" || output == "-" {
		return os.Stdout, nil, nil
	}
	file, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, nil, fmt.Errorf("创建输出文件失败: %w", err)
	}
	return file, file, nil
}

func runList(svc *service.AIEntitlementService, args []string) {
	flags := flag.NewFlagSet("activation-code list", flag.ExitOnError)
	status := flags.String("status", "all", "all | unused | redeemed | expired | revoked")
	batch := flags.String("batch", "", "按批次过滤")
	_ = flags.Parse(args)
	validStatuses := map[string]bool{"all": true, "unused": true, "redeemed": true, "expired": true, "revoked": true}
	if !validStatuses[*status] {
		log.Fatalf("不支持的状态: %s", *status)
	}

	records, err := svc.ListActivationCodes(*batch, *status)
	if err != nil {
		log.Fatalf("查询激活码失败: %v", err)
	}
	rows := make([][]string, 0, len(records))
	for _, record := range records {
		expiresText := "-"
		if record.ExpiresAt != nil {
			expiresText = record.ExpiresAt.In(time.Local).Format("2006-01-02 15:04")
		}
		userText := "-"
		if record.RedeemedByUserID != nil {
			userText = strconv.FormatUint(uint64(*record.RedeemedByUserID), 10)
		}
		rows = append(rows, []string{
			strconv.FormatUint(uint64(record.ID), 10), record.DisplaySuffix, record.BatchName, record.Status,
			expiresText, userText, record.AssignedTo, record.CreatedAt.In(time.Local).Format("2006-01-02 15:04"),
		})
	}
	if err := writeTable(os.Stdout, []tableColumn{
		{Title: "ID", AlignRight: true}, {Title: "尾号"}, {Title: "批次"}, {Title: "状态"},
		{Title: "到期时间"}, {Title: "兑换用户", AlignRight: true}, {Title: "发放对象"}, {Title: "创建时间"},
	}, rows); err != nil {
		log.Fatalf("输出激活码列表失败: %v", err)
	}
}

func safeCell(value string) string {
	return strings.NewReplacer("\t", " ", "\r", " ", "\n", " ").Replace(value)
}

func runRevoke(svc *service.AIEntitlementService, args []string) {
	flags := flag.NewFlagSet("activation-code revoke", flag.ExitOnError)
	id := flags.Uint("id", 0, "激活码记录 ID")
	_ = flags.Parse(args)
	if *id == 0 {
		log.Fatal("必须提供 --id")
	}
	if err := svc.RevokeActivationCode(*id); err != nil {
		log.Fatalf("作废失败: %v", err)
	}
	fmt.Printf("激活码 ID %d 已作废\n", *id)
}
