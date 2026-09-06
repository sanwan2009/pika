package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/pika-monitor/pika/internal/models"
	"github.com/pika-monitor/pika/internal/protocol"
	"github.com/pika-monitor/pika/internal/repo"
	"github.com/pika-monitor/pika/internal/websocket"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type PublicIPService struct {
	logger          *zap.Logger
	propertyService *PropertyService
	agentRepo       *repo.AgentRepo
	wsManager       *websocket.Manager
}

func NewPublicIPService(logger *zap.Logger, db *gorm.DB, propertyService *PropertyService, wsManager *websocket.Manager) *PublicIPService {
	return &PublicIPService{
		logger:          logger,
		propertyService: propertyService,
		agentRepo:       repo.NewAgentRepo(db),
		wsManager:       wsManager,
	}
}

// Run 启动公网 IP 采集调度
func (s *PublicIPService) Run(ctx context.Context) {
	s.logger.Info("公网 IP 采集定时任务已启动")

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("公网 IP 采集定时任务已停止")
			return
		default:
		}

		config, err := s.propertyService.GetPublicIPConfig(context.Background())
		if err != nil {
			s.logger.Error("获取公网 IP 采集配置失败", zap.Error(err))
			if !s.sleepWithCancel(ctx, time.Minute) {
				return
			}
			continue
		}

		if !config.Enabled || (!config.IPv4Enabled && !config.IPv6Enabled) {
			if !s.sleepWithCancel(ctx, 30*time.Second) {
				return
			}
			continue
		}

		interval := time.Duration(config.IntervalSeconds) * time.Second
		if interval <= 0 {
			interval = 5 * time.Minute
		}
		if interval < 30*time.Second {
			interval = 30 * time.Second
		}

		s.sendConfigToOnlineAgents(config)

		if !s.sleepWithCancel(ctx, interval) {
			return
		}
	}
}

func (s *PublicIPService) sendConfigToOnlineAgents(config *models.PublicIPConfig) {
	agentIDs := s.wsManager.GetAllClients()
	if len(agentIDs) == 0 {
		return
	}

	// 批量查询探针标签，用于按标签匹配采集范围
	agentTagMap := make(map[string][]string, len(agentIDs))
	if agents, err := s.agentRepo.FindByIdIn(context.Background(), agentIDs); err == nil {
		for _, agent := range agents {
			agentTagMap[agent.ID] = agent.Tags
		}
	}

	for _, agentID := range agentIDs {
		if agentID == "" {
			continue
		}
		agentTags := agentTagMap[agentID]
		ipv4Enabled := config.IsIPv4Target(agentID, agentTags)
		ipv6Enabled := config.IsIPv6Target(agentID, agentTags)
		if !ipv4Enabled && !ipv6Enabled {
			continue
		}

		msgData, err := json.Marshal(protocol.OutboundMessage{
			Type: protocol.MessageTypePublicIPConfig,
			Data: protocol.PublicIPConfigData{
				Enabled:         config.Enabled,
				IntervalSeconds: config.IntervalSeconds,
				IPv4Enabled:     ipv4Enabled,
				IPv6Enabled:     ipv6Enabled,
				IPv4APIs:        config.IPv4APIs,
				IPv6APIs:        config.IPv6APIs,
			},
		})
		if err != nil {
			s.logger.Error("构建公网 IP 配置消息失败", zap.Error(err))
			return
		}

		if err := s.wsManager.SendToClient(agentID, msgData); err != nil {
			s.logger.Debug("发送公网 IP 配置失败", zap.String("agentID", agentID), zap.Error(err))
		}
	}
}

func (s *PublicIPService) sleepWithCancel(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
