package service

import (
	"fmt"

	"accounter/backend/internal/model"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// SeedDefaultData 播种默认账户和分类
func SeedDefaultData(db *gorm.DB, ledgerID, userID uint) error {
	// 默认账户（按顺序，第一个为默认）
	defaultAccounts := []struct {
		Name string
		Type string
		Sort int
	}{
		{"支付宝", "alipay", 1},
		{"银行卡", "bank", 2},
		{"微信钱包", "wechat", 3},
		{"现金", "cash", 4},
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

	// 默认支出分类（一级 + 二级）
	expenseCategories := []struct {
		Name     string
		Icon     string
		Children []string
	}{
		{"餐饮", "🍽️", []string{"早餐", "午餐", "晚餐", "零食饮料"}},
		{"交通", "🚗", []string{"公交地铁", "打车", "加油", "停车"}},
		{"购物", "🛒", []string{"日用品", "服装", "电子产品", "其他"}},
		{"居住", "🏠", []string{"房租", "水电燃气", "物业费", "维修"}},
		{"娱乐", "🎮", []string{"电影", "游戏", "旅游", "运动"}},
		{"医疗", "💊", []string{"门诊", "药品", "体检"}},
		{"教育", "📚", []string{"书籍", "课程", "培训"}},
		{"通讯", "📱", []string{"话费", "网费"}},
		{"人情", "🎁", []string{"礼金", "请客", "红包"}},
		{"其他", "❓", []string{}},
	}

	for sortIdx, cat := range expenseCategories {
		// 创建一级分类
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

		// 创建二级分类
		for childIdx, childName := range cat.Children {
			child := model.Category{
				LedgerID: ledgerID,
				UserID:   userID,
				Name:     childName,
				ParentID: &parent.ID,
				Kind:     "expense",
				Sort:     childIdx + 1,
			}
			if err := db.Create(&child).Error; err != nil {
				return fmt.Errorf("创建子分类 %s 失败: %w", childName, err)
			}
		}
	}

	// 默认收入分类（一级）
	incomeCategories := []struct {
		Name string
		Icon string
	}{
		{"工资", "💰"},
		{"奖金", "🎁"},
		{"理财", "📈"},
		{"兼职", "💼"},
		{"红包", "🧧"},
		{"其他", "❓"},
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
