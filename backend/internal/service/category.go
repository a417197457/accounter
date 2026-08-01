package service

import (
	"errors"
	"fmt"

	"accounter/backend/internal/model"
	"accounter/backend/internal/repository"

	"gorm.io/gorm"
)

// CategoryService 分类服务
type CategoryService struct {
	db *gorm.DB
}

// NewCategoryService 创建分类服务
func NewCategoryService() *CategoryService {
	return &CategoryService{db: repository.GetDB()}
}

// Create 创建分类
func (s *CategoryService) Create(ledgerID, userID uint, cat *model.Category) (*model.Category, error) {
	cat.LedgerID = ledgerID
	cat.UserID = userID

	// 校验父分类归属
	if cat.ParentID != nil {
		var parent model.Category
		if err := s.db.First(&parent, "id = ? AND ledger_id = ?", *cat.ParentID, ledgerID).Error; err != nil {
			return nil, errors.New("父分类不存在")
		}
		// 子类继承父类 kind
		cat.Kind = parent.Kind
	}

	if err := s.db.Create(cat).Error; err != nil {
		return nil, fmt.Errorf("创建分类失败: %w", err)
	}
	return cat, nil
}

// Update 更新分类
func (s *CategoryService) Update(ledgerID, userID, id uint, next *model.Category) (*model.Category, error) {
	var cat model.Category
	if err := s.db.First(&cat, "id = ? AND ledger_id = ?", id, ledgerID).Error; err != nil {
		return nil, errors.New("分类不存在")
	}

	cat.Name = next.Name
	cat.Icon = next.Icon
	cat.Sort = next.Sort

	if err := s.db.Save(&cat).Error; err != nil {
		return nil, fmt.Errorf("更新分类失败: %w", err)
	}
	return &cat, nil
}

// Delete 删除分类（有流水或子分类则拒绝）
func (s *CategoryService) Delete(ledgerID, userID, id uint) error {
	// 检查是否有子分类
	var childCount int64
	s.db.Model(&model.Category{}).
		Where("ledger_id = ? AND parent_id = ? AND deleted_at IS NULL", ledgerID, id).
		Count(&childCount)
	if childCount > 0 {
		return errors.New("该分类下有子分类，无法删除")
	}

	// 检查是否有流水使用该分类
	var txCount int64
	s.db.Model(&model.Transaction{}).
		Where("ledger_id = ? AND category_id = ? AND deleted_at IS NULL", ledgerID, id).
		Count(&txCount)
	if txCount > 0 {
		return errors.New("该分类下有流水，无法删除")
	}

	if err := s.db.Delete(&model.Category{}, "id = ? AND ledger_id = ?", id, ledgerID).Error; err != nil {
		return fmt.Errorf("删除分类失败: %w", err)
	}
	return nil
}

// ListTree 获取分类树（按 kind 筛选）
func (s *CategoryService) ListTree(ledgerID uint, kind string) ([]model.Category, error) {
	var cats []model.Category
	query := s.db.Where("ledger_id = ? AND deleted_at IS NULL", ledgerID)
	if kind != "" {
		query = query.Where("kind = ?", kind)
	}
	err := query.Order("sort asc, id asc").Find(&cats).Error
	if err != nil {
		return nil, fmt.Errorf("查询分类列表失败: %w", err)
	}

	// 内存组装树
	var roots []model.Category
	childrenMap := make(map[uint][]model.Category)

	for i := range cats {
		if cats[i].ParentID == nil {
			roots = append(roots, cats[i])
		} else {
			childrenMap[*cats[i].ParentID] = append(childrenMap[*cats[i].ParentID], cats[i])
		}
	}

	// 给每个根分类挂 children
	for i := range roots {
		if children, ok := childrenMap[roots[i].ID]; ok {
			roots[i].Children = children
		}
	}

	return roots, nil
}

// ListFlat 获取扁平分类列表（供 AI 上下文用）
func (s *CategoryService) ListFlat(ledgerID uint) ([]model.Category, error) {
	var cats []model.Category
	err := s.db.Where("ledger_id = ? AND deleted_at IS NULL", ledgerID).
		Order("sort asc, id asc").
		Find(&cats).Error
	if err != nil {
		return nil, fmt.Errorf("查询分类列表失败: %w", err)
	}
	return cats, nil
}
