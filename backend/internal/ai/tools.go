package ai

import "github.com/sashabaranov/go-openai"

// toolSchemaCreate 记账工具的参数 schema
// AI 从用户话语中提取记账要素，名称用中文（账户名/分类名），后端按名称映射到 ID
var toolSchemaCreate = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"op_type": map[string]any{
			"type": "string",
			"enum": []string{"expense", "income", "borrow_in", "borrow_out", "transfer"},
			"description": "操作类型：支出=expense 收入=income 借入=borrow_in 借出=borrow_out 转账=transfer",
		},
		"amount": map[string]any{
			"type": "string",
			"description": "金额，字符串形式，如 50 或 50.00",
		},
		"account_name": map[string]any{
			"type": "string",
			"description": "账户名称，如 支付宝/银行卡/微信钱包/现金。转账时为转出账户",
		},
		"to_account_name": map[string]any{
			"type": "string",
			"description": "仅转账时：转入账户名称",
		},
		"category_name": map[string]any{
			"type": "string",
			"description": "分类名称（一级或二级），如 餐饮/早餐、交通/打车。非转账必填",
		},
		"remark": map[string]any{
			"type": "string",
			"description": "备注",
		},
		"occurred_at": map[string]any{
			"type": "string",
			"description": "日期，格式 YYYY-MM-DD。如未提及默认今天",
		},
	},
	"required": []string{"op_type", "amount", "account_name"},
}

// toolSchemaQuery 查询工具的参数 schema
var toolSchemaQuery = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"intent": map[string]any{
			"type": "string",
			"description": "查询意图摘要，如 本月支出/餐饮花了多少/最近一笔",
		},
		"kind": map[string]any{
			"type": "string",
			"enum": []string{"expense", "income"},
			"description": "查询的收支类型，可选",
		},
	},
	"required": []string{"intent"},
}

// toolDefinitions 返回 Function Calling 工具定义
func toolDefinitions() []openai.Tool {
	return []openai.Tool{
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "create_transaction",
				Description: "从用户的话里提取一笔记账信息。不要直接调用此工具后就说已记账——记账需要用户确认。",
				Parameters:  toolSchemaCreate,
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
