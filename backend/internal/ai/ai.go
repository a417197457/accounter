package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"accounter/backend/internal/config"
	"accounter/backend/internal/model"
	"accounter/backend/internal/repository"
	"accounter/backend/internal/service"

	"github.com/sashabaranov/go-openai"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// Draft 记账草稿（AI 提取，待用户确认）
type Draft struct {
	OpType        string `json:"op_type"`
	Amount        string `json:"amount"`
	AccountID     uint   `json:"account_id"`
	AccountName   string `json:"account_name"`
	CategoryID    *uint  `json:"category_id,omitempty"`
	CategoryName  string `json:"category_name,omitempty"`
	ToAccountID   *uint  `json:"to_account_id,omitempty"`
	ToAccountName string `json:"to_account_name,omitempty"`
	Remark        string `json:"remark"`
	OccurredAt    string `json:"occurred_at"`
}

// ChatResult AI 对话结果
type ChatResult struct {
	Reply string `json:"reply"` // AI 的文字回复
	Draft *Draft `json:"draft"` // 记账草稿（如果有）
}

// Service AI 服务
type Service struct {
	mu        sync.RWMutex
	client    *openai.Client
	model     string
	baseURL   string
	apiKey    string
	asrModel  string
	asrBaseURL string
	asrAPIKey  string
	db        *gorm.DB
}

// NewService 创建 AI 服务（先配对话再配语音，语音空值回落对话配置）
func NewService() *Service {
	cfg := config.Get()
	s := &Service{db: repository.GetDB()}
	s.Configure(cfg.AI.BaseURL, cfg.AI.APIKey, cfg.AI.Model)
	s.ConfigureASR(cfg.AI.ASR.BaseURL, cfg.AI.ASR.APIKey, cfg.AI.ASR.Model)
	return s
}

// Configure 配置对话模型（运行时热更新）
func (s *Service) Configure(baseURL, apiKey, model string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.baseURL = baseURL
	s.apiKey = apiKey
	s.model = model
	clientConf := openai.DefaultConfig(apiKey)
	clientConf.BaseURL = baseURL
	s.client = openai.NewClientWithConfig(clientConf)
}

// ConfigureASR 配置语音模型（空值回落对话配置）
func (s *Service) ConfigureASR(baseURL, apiKey, model string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if baseURL != "" {
		s.asrBaseURL = baseURL
	} else {
		s.asrBaseURL = s.baseURL
	}
	if apiKey != "" {
		s.asrAPIKey = apiKey
	} else {
		s.asrAPIKey = s.apiKey
	}
	s.asrModel = model
}

// IsConfigured 是否已配置对话模型
func (s *Service) IsConfigured() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.apiKey != "" && s.apiKey != "YOUR_BAILIAN_API_KEY" && s.client != nil
}

// buildSystemPrompt 构造系统提示（塞入账户/分类/今天日期）
func (s *Service) buildSystemPrompt(ledgerID uint) string {
	var accounts []model.Account
	s.db.Where("ledger_id = ? AND deleted_at IS NULL", ledgerID).
		Order("sort asc, id asc").Find(&accounts)

	var categories []model.Category
	s.db.Where("ledger_id = ? AND deleted_at IS NULL", ledgerID).
		Order("sort asc, id asc").Find(&categories)

	// 账户列表
	accountNames := []string{}
	for _, a := range accounts {
		accountNames = append(accountNames, a.Name)
	}

	// 分类列表（带层级）
	categoryNames := []string{}
	for _, c := range categories {
		if c.ParentID == nil {
			// 一级分类，列出子类
			children := []string{}
			for _, cc := range categories {
				if cc.ParentID != nil && *cc.ParentID == c.ID {
					children = append(children, cc.Name)
				}
			}
			if len(children) > 0 {
				categoryNames = append(categoryNames, c.Name+"("+strings.Join(children, "/")+")")
			} else {
				categoryNames = append(categoryNames, c.Name)
			}
		}
	}

	today := time.Now().Format("2006-01-02")

	return fmt.Sprintf(`你是一个中文记账助手。当前日期是 %s。

用户的账户有：%s
用户的分类有：%s

当用户说了一句记账的话（如"今天打车花了30"），请调用 create_transaction 工具提取信息：
- op_type：支出=expense 收入=income 借入=borrow_in 借出=borrow_out 转账=transfer
- account_name：从用户话语推断，没说就用"支付宝"
- category_name：尽量匹配上面的分类（可用一级或二级）
- amount：金额字符串
- occurred_at：YYYY-MM-DD，没说就今天

当用户查询（如"这个月花了多少"），调用 query_transactions 工具。

不要在 create_transaction 后就说"已记账"——记账需要用户确认。你应该简短回复并提示用户确认。`, today,
		strings.Join(accountNames, "、"),
		strings.Join(categoryNames, "、"))
}

// Chat 对话记账
func (s *Service) Chat(ctx context.Context, ledgerID, userID uint, message string) (*ChatResult, error) {
	if !s.IsConfigured() {
		return nil, errors.New("AI 未配置，请在 config.yaml 填入 ai.api_key")
	}

	s.mu.RLock()
	client := s.client
	modelName := s.model
	s.mu.RUnlock()

	systemPrompt := s.buildSystemPrompt(ledgerID)

	// 第一次调用
	req := openai.ChatCompletionRequest{
		Model:    modelName,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: message},
		},
		Tools:     toolDefinitions(),
		ToolChoice: "auto",
	}

	resp, err := client.CreateChatCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("AI 调用失败: %w", err)
	}
	if len(resp.Choices) == 0 {
		return nil, errors.New("AI 无响应")
	}
	choice := resp.Choices[0].Message

	// 没有 tool call，直接返回文本
	if len(choice.ToolCalls) == 0 {
		return &ChatResult{Reply: choice.Content}, nil
	}

	// 处理第一个 tool call
	tc := choice.ToolCalls[0]
	switch tc.Function.Name {
	case "create_transaction":
		draft, err := s.parseDraft(ledgerID, tc.Function.Arguments)
		if err != nil {
			return &ChatResult{Reply: "解析记账信息失败：" + err.Error()}, nil
		}
		return &ChatResult{
			Reply: fmt.Sprintf("好的，我帮你记一笔：%s%s 元（%s）。确认保存吗？",
				map[string]string{"expense": "支出", "income": "收入", "borrow_in": "借入", "borrow_out": "借出", "transfer": "转账"}[draft.OpType],
				draft.Amount, draft.AccountName),
			Draft: draft,
		}, nil
	case "query_transactions":
		reply, err := s.handleQuery(ctx, ledgerID, tc, choice, modelName)
		if err != nil {
			return &ChatResult{Reply: "查询失败：" + err.Error()}, nil
		}
		return &ChatResult{Reply: reply}, nil
	}

	return &ChatResult{Reply: choice.Content}, nil
}

// parseDraft 解析 tool call 参数为草稿（名称→ID 映射）
func (s *Service) parseDraft(ledgerID uint, argsJSON string) (*Draft, error) {
	var args struct {
		OpType        string `json:"op_type"`
		Amount        string `json:"amount"`
		AccountName   string `json:"account_name"`
		ToAccountName string `json:"to_account_name"`
		CategoryName  string `json:"category_name"`
		Remark        string `json:"remark"`
		OccurredAt    string `json:"occurred_at"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return nil, fmt.Errorf("参数解析失败: %w", err)
	}

	if args.OccurredAt == "" {
		args.OccurredAt = time.Now().Format("2006-01-02")
	}
	args.OccurredAt = normalizeTime(args.OccurredAt)

	draft := &Draft{
		OpType:      args.OpType,
		Amount:      args.Amount,
		Remark:      args.Remark,
		OccurredAt:  args.OccurredAt,
		AccountName: args.AccountName,
	}

	// 账户映射：按名称，找不到用第一个
	account, ok := s.findAccountByName(ledgerID, args.AccountName)
	if !ok {
		// 用第一个账户兜底
		s.db.Where("ledger_id = ? AND deleted_at IS NULL", ledgerID).
			Order("sort asc, id asc").First(&account)
	}
	if account.ID == 0 {
		return nil, errors.New("账本下没有可用账户")
	}
	draft.AccountID = account.ID
	draft.AccountName = account.Name

	// 转账转入账户
	if args.OpType == "transfer" && args.ToAccountName != "" {
		toAcc, _ := s.findAccountByName(ledgerID, args.ToAccountName)
		if toAcc.ID != 0 {
			draft.ToAccountID = &toAcc.ID
			draft.ToAccountName = toAcc.Name
		}
	}

	// 分类映射：按名称（一级或二级）。AI 可能返回 "父类/子类" 格式，取子类名查找
	if args.CategoryName != "" && args.OpType != "transfer" {
		catName := args.CategoryName
		// 支持 "父类/子类" → 取子类名；也支持纯父类名
		if idx := strings.LastIndex(catName, "/"); idx >= 0 {
			catName = strings.TrimSpace(catName[idx+1:])
		}
		catName = strings.TrimSpace(catName)
		cat := s.findCategoryByName(ledgerID, catName)
		if cat != nil {
			draft.CategoryID = &cat.ID
			draft.CategoryName = cat.Name
		} else {
			// 找不到子类，尝试匹配一级分类名
			cat = s.findCategoryByName(ledgerID, args.CategoryName)
			if cat != nil {
				draft.CategoryID = &cat.ID
				draft.CategoryName = cat.Name
			} else {
				draft.CategoryName = args.CategoryName
			}
		}
	}

	return draft, nil
}

// findAccountByName 按名称查账户
func (s *Service) findAccountByName(ledgerID uint, name string) (model.Account, bool) {
	var acc model.Account
	if name == "" {
		return acc, false
	}
	err := s.db.Where("ledger_id = ? AND name = ? AND deleted_at IS NULL", ledgerID, name).First(&acc).Error
	return acc, err == nil
}

// findCategoryByName 按名称查分类（一级或二级均可）
func (s *Service) findCategoryByName(ledgerID uint, name string) *model.Category {
	var cat model.Category
	err := s.db.Where("ledger_id = ? AND name = ? AND deleted_at IS NULL", ledgerID, name).First(&cat).Error
	if err != nil {
		return nil
	}
	return &cat
}

// handleQuery 处理查询：调 stats，结果喂回 LLM 生成自然语言回复
func (s *Service) handleQuery(ctx context.Context, ledgerID uint, tc openai.ToolCall, prevMessage openai.ChatCompletionMessage, modelName string) (string, error) {
	var args struct {
		Intent string `json:"intent"`
		Kind   string `json:"kind"`
	}
	_ = json.Unmarshal([]byte(tc.Function.Arguments), &args)

	// 调 stats overview + category
	statsSvc := service.NewStatsService()
	overview, _ := statsSvc.Overview(ledgerID)

	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	nextMonth := monthStart.AddDate(0, 1, 0)
	cats, _ := statsSvc.CategoryBreakdown(ledgerID, "expense", &monthStart, &nextMonth)

	// 拼接查询结果摘要
	catLines := []string{}
	for _, c := range cats {
		catLines = append(catLines, fmt.Sprintf("%s:%s", c.CategoryName, c.Amount))
	}
	summary := fmt.Sprintf("本月总收入 %s，总支出 %s，结余 %s。支出分类：%s",
		overview.Income, overview.Expense, overview.Balance, strings.Join(catLines, "、"))

	// 第二次调用 LLM，带 tool 结果
	s.mu.RLock()
	client := s.client
	s.mu.RUnlock()

	req := openai.ChatCompletionRequest{
		Model: modelName,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: s.buildSystemPrompt(ledgerID)},
			{Role: openai.ChatMessageRoleUser, Content: args.Intent},
			prevMessage,
			{Role: openai.ChatMessageRoleTool, Content: summary, ToolCallID: tc.ID},
		},
	}
	resp, err := client.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("AI 二次调用失败: %w", err)
	}
	if len(resp.Choices) == 0 {
		return summary, nil
	}
	return resp.Choices[0].Message.Content, nil
}

// ConfirmDraft 确认草稿并落库
func (s *Service) ConfirmDraft(ledgerID, userID uint, draft *Draft) (*model.Transaction, error) {
	amount, err := parseAmount(draft.Amount)
	if err != nil {
		return nil, fmt.Errorf("金额格式错误: %w", err)
	}

	occurredAt, err := time.ParseInLocation("2006-01-02 15:04:05", draft.OccurredAt, time.Local)
	if err != nil {
		// 尝试纯日期
		occurredAt, err = time.ParseInLocation("2006-01-02", draft.OccurredAt, time.Local)
		if err != nil {
			return nil, fmt.Errorf("时间格式错误: %w", err)
		}
	}

	t := &model.Transaction{
		AccountID:   draft.AccountID,
		ToAccountID: draft.ToAccountID,
		CategoryID:  draft.CategoryID,
		OpType:      draft.OpType,
		Amount:      amount,
		Remark:      draft.Remark,
		OccurredAt:  occurredAt,
	}

	txSvc := service.NewTransactionService()
	return txSvc.Create(ledgerID, userID, t)
}

// normalizeTime 把纯日期补成当天 12:00（避免 sqlite 时区偏移）
func normalizeTime(s string) string {
	if len(s) == 10 && strings.Contains(s, "-") {
		return s + " 12:00:00"
	}
	return s
}

// parseAmount 解析金额字符串为 decimal
func parseAmount(s string) (decimal.Decimal, error) {
	d, err := decimal.NewFromString(s)
	if err != nil {
		return decimal.Zero, fmt.Errorf("金额格式错误: %w", err)
	}
	return d, nil
}
