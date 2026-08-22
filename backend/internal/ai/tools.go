package ai

import "github.com/sashabaranov/go-openai"

// 单笔记账字段（create_transaction / create_transactions.items 共用）
var transactionItemProps = map[string]any{
	"op_type": map[string]any{
		"type":        "string",
		"enum":        []string{"expense", "income", "borrow_in", "borrow_out", "transfer"},
		"description": "操作类型：支出=expense 收入=income 借入=borrow_in 借出=borrow_out 转账=transfer",
	},
	"amount": map[string]any{
		"type":        "string",
		"description": "金额，字符串形式，如 50 或 50.00",
	},
	"account_name": map[string]any{
		"type":        "string",
		"description": "实际资金账户名称，如 支付宝/微信/现金/招商储蓄卡/招商信用卡。转账时为转出账户",
	},
	"payment_account_name": map[string]any{
		"type":        "string",
		"description": "可选支付渠道，仅支付宝或微信；例如支付宝绑定信用卡消费时填支付宝，account_name 填实际信用卡",
	},
	"account_specified": map[string]any{
		"type":        "boolean",
		"description": "用户的说明是否满足当前账户策略。便捷模式：明确说支付宝/微信时可按余额处理并填 true，完全没说账户才为 false；精确模式：只有明确到支付宝/微信余额、渠道关联的具体卡，或直接使用的现金/具体银行卡时才为 true",
	},
	"to_account_name": map[string]any{
		"type":        "string",
		"description": "仅转账时：转入账户名称",
	},
	"category_name": map[string]any{
		"type":        "string",
		"description": "分类名称（一级或二级），如 餐饮/早餐、交通/打车。非转账必填",
	},
	"remark": map[string]any{
		"type":        "string",
		"description": "备注，可摘录原话中的商户/事项",
	},
	"occurred_at": map[string]any{
		"type":        "string",
		"description": "发生时间，格式 YYYY-MM-DD HH:mm:ss。未提及日期和时间时使用系统提示中的当前时间；只说日期时沿用当前时分秒",
	},
}

var toolSchemaCreate = map[string]any{
	"type":       "object",
	"properties": transactionItemProps,
	"required":   []string{"op_type", "amount", "account_name", "account_specified"},
}

var toolSchemaCreateBatch = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"items": map[string]any{
			"type":        "array",
			"description": "从一段话中拆出的多笔记账，按出现顺序排列",
			"items": map[string]any{
				"type":       "object",
				"properties": transactionItemProps,
				"required":   []string{"op_type", "amount", "account_name", "account_specified"},
			},
		},
	},
	"required": []string{"items"},
}

var toolSchemaQuery = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"intent": map[string]any{
			"type":        "string",
			"description": "查询意图摘要，如 本月支出/餐饮花了多少/最近一笔",
		},
		"kind": map[string]any{
			"type":        "string",
			"enum":        []string{"expense", "income"},
			"description": "查询的收支类型，可选",
		},
	},
	"required": []string{"intent"},
}

func toolDefinitions() []openai.Tool {
	return []openai.Tool{
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "create_transaction",
				Description: "提取单笔记账。若用户一段话里有多笔消费/收入，请优先用 create_transactions，或对本工具调用多次（每笔一次）。不要直接说已记账——需用户确认。",
				Parameters:  toolSchemaCreate,
			},
		},
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "create_transactions",
				Description: "从用户一段话或长语音里批量提取多笔记账（推荐）。例如「中午外卖35，下午打车22，晚上买菜58」应拆成 3 条。不要说已记账——需用户确认。",
				Parameters:  toolSchemaCreateBatch,
			},
		},
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "query_transactions",
				Description: "查询用户的流水/统计，如本月支出、某分类花了多少。",
				Parameters:  toolSchemaQuery,
			},
		},
	}
}
