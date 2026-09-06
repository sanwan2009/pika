package repo

import (
	"context"

	"github.com/go-orz/orz"
	"github.com/pika-monitor/pika/internal/models"
	"gorm.io/gorm"
)

type AlertRuleRepo struct {
	orz.Repository[models.AlertRule, string]
	db *gorm.DB
}

func NewAlertRuleRepo(db *gorm.DB) *AlertRuleRepo {
	return &AlertRuleRepo{
		Repository: orz.NewRepository[models.AlertRule, string](db),
		db:         db,
	}
}

// Count 查询规则总数
func (r *AlertRuleRepo) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.AlertRule{}).
		Count(&count).Error
	return count, err
}

// FindEnabledRulesSorted 查询所有启用的告警规则（按优先级升序、创建时间升序）
func (r *AlertRuleRepo) FindEnabledRulesSorted(ctx context.Context) ([]models.AlertRule, error) {
	var rules []models.AlertRule
	err := r.db.WithContext(ctx).
		Where("enabled = ?", true).
		Order("priority ASC, created_at ASC").
		Find(&rules).Error
	return rules, err
}
