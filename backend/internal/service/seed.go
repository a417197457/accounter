package service

import (
	"fmt"

	"accounter/backend/internal/model"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// SeedDefaultData 播种默认账户和分类
func SeedDefaultData(db *gorm.DB, ledgerID, userID uint) error {
	defaultAccounts := []struct {
		Name string
		Type string
		Sort int
	}{
		{"支付宝", "alipay", 1},
		{"微信", "wechat", 2},
		{"现金", "cash", 3},
		{"储蓄卡", "savings", 4},
		{"信用卡", "credit", 5},
	}

	for _, acc := range defaultAccounts {
		account := model.Account{
			LedgerID: ledgerID,
			UserID:   userID,
			Name:     acc.Name,
			Type:     acc.Type,
			Balance:  decimal.Zero,
			Sort:     acc.Sort,
		}
		if err := db.Create(&account).Error; err != nil {
			return fmt.Errorf("创建账户 %s 失败: %w", acc.Name, err)
		}
	}

	expenseCategories := []struct {
		Name     string
		Icon     string
		Children []struct {
			Name string
			Icon string
		}
	}{
		{"餐饮", "🍽️", []struct{ Name, Icon string }{
			{"早餐", "🥐"}, {"午餐", "🍱"}, {"晚餐", "🍜"}, {"零食饮料", "🥤"},
		}},
		{"交通", "🚗", []struct{ Name, Icon string }{
			{"公交地铁", "🚇"}, {"打车", "🚕"}, {"加油", "⛽"}, {"停车", "🅿️"},
		}},
		{"购物", "🛒", []struct{ Name, Icon string }{
			{"日用品", "🧴"}, {"服装", "👕"}, {"电子产品", "💻"}, {"其他购物", "🛍️"},
		}},
		{"居住", "🏠", []struct{ Name, Icon string }{
			{"房租", "🔑"}, {"水电燃气", "💡"}, {"物业费", "🏢"}, {"维修", "🔧"},
		}},
		{"娱乐", "🎮", []struct{ Name, Icon string }{
			{"电影", "🎬"}, {"游戏", "🕹️"}, {"旅游", "✈️"}, {"运动", "⚽"},
		}},
		{"医疗", "💊", []struct{ Name, Icon string }{
			{"门诊", "🏥"}, {"药品", "💉"}, {"体检", "🩺"},
		}},
		{"教育", "📚", []struct{ Name, Icon string }{
			{"书籍", "📖"}, {"课程", "🎓"}, {"培训", "✏️"},
		}},
		{"通讯", "📱", []struct{ Name, Icon string }{
			{"话费", "📞"}, {"网费", "📶"},
		}},
		{"人情", "🎁", []struct{ Name, Icon string }{
			{"礼金", "🧧"}, {"请客", "🥂"}, {"红包", "💌"},
		}},
		{"其他", "📦", nil},
	}

	for sortIdx, cat := range expenseCategories {
		parent := model.Category{
			LedgerID: ledgerID,
			UserID:   userID,
			Name:     cat.Name,
			Kind:     "expense",
			Icon:     cat.Icon,
			Sort:     sortIdx + 1,
		}
		if err := db.Create(&parent).Error; err != nil {
			return fmt.Errorf("创建分类 %s 失败: %w", cat.Name, err)
		}

		for childIdx, child := range cat.Children {
			c := model.Category{
				LedgerID: ledgerID,
				UserID:   userID,
				Name:     child.Name,
				ParentID: &parent.ID,
				Kind:     "expense",
				Icon:     child.Icon,
				Sort:     childIdx + 1,
			}
			if err := db.Create(&c).Error; err != nil {
				return fmt.Errorf("创建子分类 %s 失败: %w", child.Name, err)
			}
		}
	}

	incomeCategories := []struct {
		Name string
		Icon string
	}{
		{"工资", "💰"},
		{"奖金", "🎁"},
		{"理财", "📈"},
		{"兼职", "💼"},
		{"红包", "🧧"},
		{"其他", "📦"},
	}

	for sortIdx, cat := range incomeCategories {
		category := model.Category{
			LedgerID: ledgerID,
			UserID:   userID,
			Name:     cat.Name,
			Kind:     "income",
			Icon:     cat.Icon,
			Sort:     sortIdx + 1,
		}
		if err := db.Create(&category).Error; err != nil {
			return fmt.Errorf("创建收入分类 %s 失败: %w", cat.Name, err)
		}
	}

	return nil
}
