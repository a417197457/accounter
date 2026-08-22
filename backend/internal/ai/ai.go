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
	OpType             string `json:"op_type"`
	Amount             string `json:"amount"`
	AccountID          uint   `json:"account_id"`
	AccountName        string `json:"account_name"`
	PaymentAccountID   *uint  `json:"payment_account_id,omitempty"`
	PaymentAccountName string `json:"payment_account_name,omitempty"`
	AccountSpecified   bool   `json:"account_specified"`
	CategoryID         *uint  `json:"category_id,omitempty"`
	CategoryName       string `json:"category_name,omitempty"`
	ToAccountID        *uint  `json:"to_account_id,omitempty"`
	ToAccountName      string `json:"to_account_name,omitempty"`
	Remark             string `json:"remark"`
	OccurredAt         string `json:"occurred_at"`
}

// ChatResult AI 对话结果
type ChatResult struct {
	Reply    string   `json:"reply"`              // AI 的文字回复
	Draft    *Draft   `json:"draft,omitempty"`    // 兼容：首笔草稿
	Drafts   []*Draft `json:"drafts,omitempty"`   // 多笔草稿（一段话多条消费）
	Warnings []string `json:"warnings,omitempty"` // 如信用卡临近还款提示
}

// HistoryMessage 多轮历史（不含本轮 user message）
type HistoryMessage struct {
	Role    string `json:"role"` // user | assistant
	Content string `json:"content"`
}

const maxChatHistory = 12

var errAccountDetailRequired = errors.New("请补充这笔账的具体支付账户")

// Service AI 服务
type Service struct {
	mu         sync.RWMutex
	client     *openai.Client
	model      string
	baseURL    string
	apiKey     string
	asrModel   string
	asrBaseURL string
	asrAPIKey  string
	db         *gorm.DB
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
func (s *Service) buildSystemPrompt(ledgerID uint, preference *service.BookkeepingPreferenceView) string {
	var accounts []model.Account
	s.db.Where("ledger_id = ? AND deleted_at IS NULL", ledgerID).
		Order("sort asc, id asc").Find(&accounts)
	var links []model.AccountLink
	s.db.Where("ledger_id = ?", ledgerID).Find(&links)
	accountByID := map[uint]model.Account{}
	for _, account := range accounts {
		accountByID[account.ID] = account
	}

	var categories []model.Category
	s.db.Where("ledger_id = ? AND deleted_at IS NULL", ledgerID).
		Order("sort asc, id asc").Find(&categories)

	// 账户列表
	accountNames := []string{}
	for _, a := range accounts {
		accountNames = append(accountNames, a.Name)
		for _, link := range links {
			if link.PaymentAccountID == a.ID {
				if linked, ok := accountByID[link.FundingAccountID]; ok {
					accountNames = append(accountNames, a.Name+"关联"+linked.Name)
				}
			}
		}
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

	now := time.Now()
	currentTime := now.Format("2006-01-02 15:04:05")
	accountPolicy := "【账户策略：便捷模式】未明确账户时 account_specified=false，由后端使用默认账户；不要擅自猜测。"
	if preference != nil && preference.AccountMode == "strict" {
		accountPolicy = `【账户策略：精确模式】用户必须明确完整扣款路径：支付宝/微信余额、支付宝/微信关联的具体卡，或直接使用的现金/具体银行卡。
- 如果完全没说账户，只能简短追问，不能调用创建工具。
- 如果只说“支付宝”或“微信”，且该渠道有余额和关联卡等多个来源，仍要追问“余额还是哪张卡”。
- 只有完整路径明确后 account_specified 才能为 true。不能根据常用习惯、账户顺序或历史无关账单猜测。`
	} else if preference != nil {
		accountPolicy = fmt.Sprintf(`【账户策略：便捷模式】默认支付路径是「%s」。
- 用户明确说了支付宝或微信但没说关联卡时，直接记为该渠道余额，account_specified=true；明确具体卡或完整路径时也按本次说明记录。
- 用户完全未说明任何账户或支付渠道时，account_specified=false；后端会强制改为默认支付路径。不要追问，也不要自行选择其他账户。`, preference.DefaultAccountLabel)
	}

	return fmt.Sprintf(`你是一个中文记账助手。当前时间是 %s。

用户的账户有：%s
用户的分类有：%s

%s

你能看到多轮对话历史。用户可能先说记账，你追问后，下一句是补充（如「微信」「昨天」）——必须结合历史理解，不要把补充当成全新无关消息。

【单笔】如「今天打车花了30」→ 信息足够时调用 create_transaction。
【多笔】用户可能说一大段话或念很多条消费，例如：
「中午点外卖花了35，下午打车22，晚上超市买菜花了58，用的都是支付宝」
这时必须拆成多条，优先调用 create_transactions（items 数组），也可对 create_transaction 调用多次。
每一笔都要尽量补全：金额、分类、账户、日期、备注（可摘原话里的事项）。

【追问】金额缺失或分类严重不确定、无法合理推断时：本轮只用自然语言简短追问，不要调用 create_transaction / create_transactions。账户是否追问必须严格遵守上面的账户策略。
【账户】本轮是在回答你刚刚的账户追问时，可以结合紧邻的对话补全；不要沿用更早的无关账单账户。

字段约定：
- op_type：支出=expense 收入=income 借入=borrow_in 借出=borrow_out 转账=transfer
- account_name：从话语与历史推断
- payment_account_name：仅当明确「支付宝/微信绑某张卡」时填写；account_name 填实际扣款卡
- account_specified：按上面的当前账户策略如实填写，不得因为系统设置了默认账户就填 true
- category_name：尽量匹配上面的分类（一级或二级）
- amount：金额字符串
- occurred_at：YYYY-MM-DD HH:mm:ss；用户没说日期和时间时使用当前时间；只说日期没说几点时，日期按用户描述、时分秒沿用当前时间；明确说了几点时按用户描述填写
- remark：简短事项，如外卖、打车、买菜

当用户查询（如「这个月花了多少」），调用 query_transactions。

不要说「已记账」——需要用户确认。请简短回复并提示确认（多笔时说明识别了几笔）。`, currentTime,
		strings.Join(accountNames, "、"),
		strings.Join(categoryNames, "、"),
		accountPolicy)
}

// buildChatMessages 组装 system + 近期历史 + 本轮 user
func buildChatMessages(systemPrompt, message string, history []HistoryMessage) []openai.ChatCompletionMessage {
	msgs := make([]openai.ChatCompletionMessage, 0, 2+len(history))
	msgs = append(msgs, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleSystem,
		Content: systemPrompt,
	})

	trimmed := history
	if len(trimmed) > maxChatHistory {
		trimmed = trimmed[len(trimmed)-maxChatHistory:]
	}
	for _, h := range trimmed {
		role := strings.TrimSpace(strings.ToLower(h.Role))
		content := strings.TrimSpace(h.Content)
		if content == "" {
			continue
		}
		switch role {
		case "user":
			msgs = append(msgs, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: content})
		case "assistant", "ai":
			msgs = append(msgs, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: content})
		}
	}

	msgs = append(msgs, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: message,
	})
	return msgs
}

// Chat 对话记账（history 为不含本轮 message 的近期对话）
func (s *Service) Chat(ctx context.Context, ledgerID, userID uint, message string, history []HistoryMessage) (*ChatResult, error) {
	if !s.IsConfigured() {
		return nil, errors.New("AI 未配置，请在 config.yaml 填入 ai.api_key")
	}

	s.mu.RLock()
	client := s.client
	modelName := s.model
	s.mu.RUnlock()

	preference, err := service.NewBookkeepingPreferenceService().Get(ledgerID, userID)
	if err != nil {
		return nil, err
	}
	systemPrompt := s.buildSystemPrompt(ledgerID, preference)

	req := openai.ChatCompletionRequest{
		Model:      modelName,
		Messages:   buildChatMessages(systemPrompt, message, history),
		Tools:      toolDefinitions(),
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

	drafts := make([]*Draft, 0, len(choice.ToolCalls))
	var queryReply string
	var parseErrs []string
	needsAccountDetail := false

	for _, tc := range choice.ToolCalls {
		switch tc.Function.Name {
		case "create_transaction":
			draft, err := s.parseDraft(ledgerID, tc.Function.Arguments, preference)
			if err != nil {
				if errors.Is(err, errAccountDetailRequired) {
					needsAccountDetail = true
				}
				parseErrs = append(parseErrs, err.Error())
				continue
			}
			drafts = append(drafts, draft)
		case "create_transactions":
			batch, err := s.parseDraftBatch(ledgerID, tc.Function.Arguments, preference)
			if err != nil {
				if errors.Is(err, errAccountDetailRequired) {
					needsAccountDetail = true
				}
				parseErrs = append(parseErrs, err.Error())
				continue
			}
			drafts = append(drafts, batch...)
		case "query_transactions":
			reply, err := s.handleQuery(ctx, ledgerID, tc, choice, modelName, preference)
			if err != nil {
				queryReply = "查询失败：" + err.Error()
			} else {
				queryReply = reply
			}
		}
	}
	if needsAccountDetail {
		return &ChatResult{Reply: "这笔账是从哪里支付的？请说明支付宝/微信余额、支付渠道关联的具体卡，或直接使用的现金/银行卡。"}, nil
	}

	if len(drafts) > 0 {
		warnings := s.creditDueWarnings(ledgerID, drafts)
		reply := buildDraftsReply(drafts, choice.Content)
		if len(warnings) > 0 {
			reply = reply + "\n⚠️ " + strings.Join(warnings, "；")
		}
		return &ChatResult{
			Reply:    reply,
			Draft:    drafts[0],
			Drafts:   drafts,
			Warnings: warnings,
		}, nil
	}
	if queryReply != "" {
		return &ChatResult{Reply: queryReply}, nil
	}
	if len(parseErrs) > 0 {
		return &ChatResult{Reply: "解析记账信息失败：" + strings.Join(parseErrs, "；")}, nil
	}
	return &ChatResult{Reply: choice.Content}, nil
}

// creditDueWarnings 草稿涉及信用卡且距还款日不足 3 天时提示
func (s *Service) creditDueWarnings(ledgerID uint, drafts []*Draft) []string {
	ids := map[uint]bool{}
	for _, d := range drafts {
		if d == nil {
			continue
		}
		if d.AccountID > 0 {
			ids[d.AccountID] = true
		}
		if d.ToAccountID != nil && *d.ToAccountID > 0 {
			ids[*d.ToAccountID] = true
		}
	}
	if len(ids) == 0 {
		return nil
	}
	idList := make([]uint, 0, len(ids))
	for id := range ids {
		idList = append(idList, id)
	}
	var accounts []model.Account
	if err := s.db.Where("ledger_id = ? AND id IN ? AND type = ? AND due_day IS NOT NULL", ledgerID, idList, "credit").
		Find(&accounts).Error; err != nil {
		return nil
	}
	now := time.Now()
	var warnings []string
	seen := map[uint]bool{}
	for _, acc := range accounts {
		if acc.DueDay == nil || seen[acc.ID] {
			continue
		}
		seen[acc.ID] = true
		days := model.DaysUntilDue(*acc.DueDay, now)
		if days >= 0 && days < 3 {
			due := model.NextDueDate(*acc.DueDay, now)
			if days == 0 {
				warnings = append(warnings, fmt.Sprintf("「%s」今天是还款日（%s），记得及时还款", acc.Name, due.Format("01-02")))
			} else {
				warnings = append(warnings, fmt.Sprintf("「%s」距还款日仅 %d 天（%s）", acc.Name, days, due.Format("01-02")))
			}
		}
	}
	return warnings
}

func buildDraftsReply(drafts []*Draft, modelContent string) string {
	if strings.TrimSpace(modelContent) != "" && len(drafts) == 1 {
		return modelContent
	}
	labels := map[string]string{
		"expense": "支出", "income": "收入", "borrow_in": "借入", "borrow_out": "借出", "transfer": "转账",
	}
	if len(drafts) == 1 {
		d := drafts[0]
		return fmt.Sprintf("好的，我帮你记一笔：%s %s 元（%s）。确认保存吗？",
			labels[d.OpType], d.Amount, d.AccountName)
	}
	var total decimal.Decimal
	for _, d := range drafts {
		if a, err := parseAmount(d.Amount); err == nil {
			total = total.Add(a)
		}
	}
	return fmt.Sprintf("这段话里我识别到 %d 笔记账，合计约 %s 元。请逐笔确认或一键全部保存。",
		len(drafts), total.StringFixed(2))
}

// parseDraftBatch 解析批量工具参数
func (s *Service) parseDraftBatch(ledgerID uint, argsJSON string, preference *service.BookkeepingPreferenceView) ([]*Draft, error) {
	var args struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return nil, fmt.Errorf("批量参数解析失败: %w", err)
	}
	if len(args.Items) == 0 {
		return nil, errors.New("未识别到任何记账条目")
	}
	out := make([]*Draft, 0, len(args.Items))
	for i, raw := range args.Items {
		d, err := s.parseDraft(ledgerID, string(raw), preference)
		if err != nil {
			return nil, fmt.Errorf("第 %d 笔: %w", i+1, err)
		}
		out = append(out, d)
	}
	return out, nil
}

// parseDraft 解析 tool call 参数为草稿（名称→ID 映射）
func (s *Service) parseDraft(ledgerID uint, argsJSON string, preference *service.BookkeepingPreferenceView) (*Draft, error) {
	var args struct {
		OpType             string `json:"op_type"`
		Amount             string `json:"amount"`
		AccountName        string `json:"account_name"`
		PaymentAccountName string `json:"payment_account_name"`
		AccountSpecified   bool   `json:"account_specified"`
		ToAccountName      string `json:"to_account_name"`
		CategoryName       string `json:"category_name"`
		Remark             string `json:"remark"`
		OccurredAt         string `json:"occurred_at"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return nil, fmt.Errorf("参数解析失败: %w", err)
	}

	if args.OccurredAt == "" {
		args.OccurredAt = time.Now().Format("2006-01-02 15:04:05")
	}
	args.OccurredAt = normalizeTime(args.OccurredAt)

	draft := &Draft{
		OpType:           args.OpType,
		Amount:           args.Amount,
		Remark:           args.Remark,
		OccurredAt:       args.OccurredAt,
		AccountName:      args.AccountName,
		AccountSpecified: args.AccountSpecified,
	}

	if !args.AccountSpecified {
		if preference == nil || preference.AccountMode == "strict" {
			return nil, errAccountDetailRequired
		}
		draft.AccountID = preference.DefaultAccountID
		draft.AccountName = preference.DefaultAccountName
		draft.PaymentAccountID = preference.DefaultPaymentAccountID
		draft.PaymentAccountName = preference.DefaultPaymentAccountName
	} else {
		// 用户明确账户时必须精确匹配，不能再静默回退到任意账户。
		account, ok := s.findAccountByName(ledgerID, args.AccountName)
		if !ok {
			return nil, fmt.Errorf("未找到用户说明的账户「%s」，请重新选择", args.AccountName)
		}
		draft.AccountID = account.ID
		draft.AccountName = account.Name
	}
	if args.AccountSpecified && args.PaymentAccountName != "" {
		payment, ok := s.findAccountByName(ledgerID, args.PaymentAccountName)
		if ok && (payment.Type == "alipay" || payment.Type == "wechat") {
			var count int64
			s.db.Model(&model.AccountLink{}).Where("ledger_id = ? AND payment_account_id = ? AND funding_account_id = ?", ledgerID, payment.ID, draft.AccountID).Count(&count)
			if count > 0 {
				draft.PaymentAccountID = &payment.ID
				draft.PaymentAccountName = payment.Name
			} else {
				return nil, errors.New("所选支付渠道没有关联该银行卡")
			}
		} else {
			return nil, errors.New("未找到用户说明的支付宝或微信支付渠道")
		}
	}

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
		categoryKind := categoryKindForOpType(args.OpType)
		cat := s.findCategoryByName(ledgerID, catName, categoryKind)
		if cat != nil {
			draft.CategoryID = &cat.ID
			draft.CategoryName = cat.Name
		} else {
			// 找不到子类，尝试匹配一级分类名
			cat = s.findCategoryByName(ledgerID, args.CategoryName, categoryKind)
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

func categoryKindForOpType(opType string) string {
	if opType == "income" || opType == "borrow_in" {
		return "income"
	}
	if opType == "expense" || opType == "borrow_out" {
		return "expense"
	}
	return ""
}

// findCategoryByName 按名称和收支类型查分类（一级或二级均可）。
func (s *Service) findCategoryByName(ledgerID uint, name, kind string) *model.Category {
	var cat model.Category
	query := s.db.Where("ledger_id = ? AND name = ? AND deleted_at IS NULL", ledgerID, name)
	if kind != "" {
		query = query.Where("kind = ?", kind)
	}
	err := query.First(&cat).Error
	if err != nil {
		return nil
	}
	return &cat
}

// handleQuery 处理查询：调 stats，结果喂回 LLM 生成自然语言回复
func (s *Service) handleQuery(ctx context.Context, ledgerID uint, tc openai.ToolCall, prevMessage openai.ChatCompletionMessage, modelName string, preference *service.BookkeepingPreferenceView) (string, error) {
	var args struct {
		Intent string `json:"intent"`
		Kind   string `json:"kind"`
	}
	_ = json.Unmarshal([]byte(tc.Function.Arguments), &args)

	// 调 stats overview + category
	statsSvc := service.NewStatsService()
	overview, _ := statsSvc.Overview(ledgerID, nil, nil)

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
			{Role: openai.ChatMessageRoleSystem, Content: s.buildSystemPrompt(ledgerID, preference)},
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
		AccountID:        draft.AccountID,
		PaymentAccountID: draft.PaymentAccountID,
		ToAccountID:      draft.ToAccountID,
		CategoryID:       draft.CategoryID,
		OpType:           draft.OpType,
		Amount:           amount,
		Remark:           draft.Remark,
		OccurredAt:       occurredAt,
	}

	txSvc := service.NewTransactionService()
	return txSvc.Create(ledgerID, userID, t)
}

// ConfirmDrafts 批量确认草稿并落库
func (s *Service) ConfirmDrafts(ledgerID, userID uint, drafts []*Draft) ([]*model.Transaction, error) {
	if len(drafts) == 0 {
		return nil, errors.New("没有待确认的草稿")
	}
	preference, err := service.NewBookkeepingPreferenceService().Get(ledgerID, userID)
	if err != nil {
		return nil, err
	}
	out := make([]*model.Transaction, 0, len(drafts))
	for i, d := range drafts {
		if d == nil {
			return nil, fmt.Errorf("第 %d 笔草稿为空", i+1)
		}
		if preference.AccountMode == "strict" && !d.AccountSpecified {
			return nil, fmt.Errorf("第 %d 笔缺少明确的支付账户，请先补充", i+1)
		}
		tx, err := s.ConfirmDraft(ledgerID, userID, d)
		if err != nil {
			return nil, fmt.Errorf("第 %d 笔保存失败: %w", i+1, err)
		}
		out = append(out, tx)
	}
	return out, nil
}

// normalizeTime 给 AI 返回的纯日期补上当前时分秒，避免实时记账全部显示为 12:00。
func normalizeTime(s string) string {
	return normalizeTimeAt(s, time.Now())
}

func normalizeTimeAt(s string, now time.Time) string {
	s = strings.TrimSpace(s)
	if len(s) == 10 && strings.Contains(s, "-") {
		return s + " " + now.Format("15:04:05")
	}
	if len(s) == 16 && strings.Contains(s, "-") && strings.Contains(s, ":") {
		return s + ":00"
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
