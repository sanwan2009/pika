package service

import (
	"context"

	"github.com/pika-monitor/pika/internal/models"
	"github.com/pika-monitor/pika/internal/repo"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const (
	NotificationTypeTraffic     = "traffic"
	NotificationTypeSSHLogin    = "ssh_login"
	NotificationTypeTamperEvt   = "tamper"
	NotificationTypeAgentExpire = "agent_expire"
)

// NotificationService 统一通知发送入口（事件类通知，按各主机命中的告警规则驱动）
type NotificationService struct {
	logger           *zap.Logger
	propertyService  *PropertyService
	alertRuleService *AlertRuleService
	alertRecordRepo  *repo.AlertRecordRepo
	notifier         *Notifier
}

func NewNotificationService(logger *zap.Logger, db *gorm.DB, propertyService *PropertyService, alertRuleService *AlertRuleService, notifier *Notifier) *NotificationService {
	return &NotificationService{
		logger:           logger,
		propertyService:  propertyService,
		alertRuleService: alertRuleService,
		alertRecordRepo:  repo.NewAlertRecordRepo(db),
		notifier:         notifier,
	}
}

// SendAlertNotification 根据主机命中的告警规则发送事件通知
func (s *NotificationService) SendAlertNotification(ctx context.Context, notificationType string, record *models.AlertRecord, agent *models.Agent) error {
	// 解析该主机命中的告警规则（未命中任何规则时不发送事件通知）
	config, err := s.alertRuleService.ResolveForAgent(ctx, agent.ID)
	if err != nil {
		return err
	}
	if config == nil {
		return nil
	}

	if !isNotificationEnabled(config.Notifications, notificationType) {
		return nil
	}

	// 补充记录的规则归属并持久化（事件记录在调用方创建，创建时未知命中规则）
	if record.ConfigID == "" {
		record.ConfigID = config.ConfigID
		record.ConfigName = config.Name
		if record.ID > 0 {
			if err := s.alertRecordRepo.UpdateAlertRecord(ctx, record); err != nil {
				s.logger.Warn("更新事件记录规则归属失败", zap.Int64("recordId", record.ID), zap.Error(err))
			}
		}
	}

	channelConfigs, err := s.propertyService.GetNotificationChannelConfigs(ctx)
	if err != nil {
		return err
	}

	enabledChannels := filterChannelsByTypes(channelConfigs, config.Channels)
	if len(enabledChannels) == 0 {
		return nil
	}

	if err := s.notifier.SendNotificationByConfigs(ctx, enabledChannels, record, agent, config.MaskIP); err != nil {
		s.logger.Error("发送通知失败", zap.Error(err))
		return err
	}

	return nil
}

// ResolveMaskIP 解析主机命中规则的 IP 打码配置（未命中规则时不打码）
func (s *NotificationService) ResolveMaskIP(ctx context.Context, agentID string) bool {
	config, err := s.alertRuleService.ResolveForAgent(ctx, agentID)
	if err != nil || config == nil {
		return false
	}
	return config.MaskIP
}

// filterChannelsByTypes 过滤启用的通知渠道并按渠道类型筛选（types 为空时返回所有启用渠道）
func filterChannelsByTypes(channelConfigs []models.NotificationChannelConfig, types []string) []models.NotificationChannelConfig {
	var enabled []models.NotificationChannelConfig
	typeSet := make(map[string]struct{}, len(types))
	for _, t := range types {
		typeSet[t] = struct{}{}
	}

	for _, channel := range channelConfigs {
		if !channel.Enabled {
			continue
		}
		if len(typeSet) > 0 {
			if _, ok := typeSet[channel.Type]; !ok {
				continue
			}
		}
		enabled = append(enabled, channel)
	}
	return enabled
}

func isNotificationEnabled(notifications models.AlertNotifications, notificationType string) bool {
	switch notificationType {
	case NotificationTypeTraffic:
		return notifications.TrafficEnabled
	case NotificationTypeSSHLogin:
		return notifications.SSHLoginSuccessEnabled
	case NotificationTypeTamperEvt:
		return notifications.TamperEventEnabled
	case NotificationTypeAgentExpire:
		return notifications.AgentExpireEnabled
	default:
		return true
	}
}
