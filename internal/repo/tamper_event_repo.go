package repo

import (
	"context"

	"github.com/go-orz/orz"
	"github.com/pika-monitor/pika/internal/models"
	"gorm.io/gorm"
)

type TamperEventRepo struct {
	orz.Repository[models.TamperEvent, string]
}

func NewTamperEventRepo(db *gorm.DB) *TamperEventRepo {
	return &TamperEventRepo{
		Repository: orz.NewRepository[models.TamperEvent, string](db),
	}
}

// DeleteEventsByAgentID 删除探针的所有防篡改事件
func (r *TamperEventRepo) DeleteEventsByAgentID(ctx context.Context, agentID string) error {
	return r.GetDB(ctx).Where("agent_id = ?", agentID).Delete(&models.TamperEvent{}).Error
}
