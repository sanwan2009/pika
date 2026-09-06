package repo

import (
	"context"

	"github.com/go-orz/orz"
	"github.com/pika-monitor/pika/internal/models"
	"gorm.io/gorm"
)

// SSHLoginEventRepo SSH登录事件数据访问层
type SSHLoginEventRepo struct {
	orz.Repository[models.SSHLoginEvent, string]
}

// NewSSHLoginEventRepo 创建仓库
func NewSSHLoginEventRepo(db *gorm.DB) *SSHLoginEventRepo {
	return &SSHLoginEventRepo{
		Repository: orz.NewRepository[models.SSHLoginEvent, string](db),
	}
}

// DeleteEventsByAgentID 删除探针的所有登录事件
func (r *SSHLoginEventRepo) DeleteEventsByAgentID(ctx context.Context, agentID string) error {
	return r.GetDB(ctx).Where("agent_id = ?", agentID).Delete(&models.SSHLoginEvent{}).Error
}
