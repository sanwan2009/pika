package service

import (
	"context"
	"fmt"
	"time"

	"github.com/go-orz/orz"
	"github.com/pika-monitor/pika/internal/models"
	"github.com/pika-monitor/pika/internal/protocol"
	"github.com/pika-monitor/pika/internal/repo"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const (
	agentExpireAlertType = "agent_expire"
)

// 机器到期提醒采用固定里程碑，避免规则配置过于复杂；每个里程碑只提醒一次。
var agentExpireReminderDays = [...]int{7, 3, 1}

// AlertService 告警服务
type AlertService struct {
	Service           *orz.Service
	AlertRecordRepo   *repo.AlertRecordRepo
	AlertStateRepo    *repo.AlertStateRepo
	agentRepo         *repo.AgentRepo
	monitorService    *MonitorService
	propertyService   *PropertyService
	alertRuleService  *AlertRuleService
	notifier          *Notifier
	notificationQueue *NotificationQueue
	logger            *zap.Logger
}

func NewAlertService(logger *zap.Logger, db *gorm.DB, propertyService *PropertyService, alertRuleService *AlertRuleService, monitorService *MonitorService, notifier *Notifier) *AlertService {
	service := &AlertService{
		Service:          orz.NewService(db),
		AlertRecordRepo:  repo.NewAlertRecordRepo(db),
		AlertStateRepo:   repo.NewAlertStateRepo(db),
		agentRepo:        repo.NewAgentRepo(db),
		monitorService:   monitorService,
		propertyService:  propertyService,
		alertRuleService: alertRuleService,
		notifier:         notifier,
		logger:           logger,
	}

	service.notificationQueue = NewNotificationQueue(db, service, service.AlertRecordRepo, logger)
	service.notificationQueue.Start()

	return service
}

// Clear 清空告警记录
func (s *AlertService) Clear(ctx context.Context) error {
	return s.Service.Transaction(ctx, func(ctx context.Context) error {
		// 清空告警记录
		if err := s.AlertRecordRepo.Clear(ctx); err != nil {
			s.logger.Error("清空告警记录失败", zap.Error(err))
			return err
		}

		// 清空告警状态
		if err := s.AlertStateRepo.Clear(ctx); err != nil {
			s.logger.Error("清空告警状态失败", zap.Error(err))
			return err
		}

		return nil
	})
}

// Shutdown 关闭告警服务
func (s *AlertService) Shutdown() {
	s.logger.Info("关闭告警服务")
	s.notificationQueue.Shutdown()
}

// CheckMetrics 检查指标并触发告警
func (s *AlertService) CheckMetrics(ctx context.Context, agentID string, cpu, memory, disk, networkSpeed float64) error {
	// 解析该主机生效的告警配置（按优先级命中的第一条规则，未命中任何规则时不告警）
	config, err := s.alertRuleService.ResolveForAgent(ctx, agentID)
	if err != nil {
		s.logger.Error("解析告警配置失败", zap.String("agentId", agentID), zap.Error(err))
		return err
	}

	// 未命中任何规则的主机不产生告警
	if config == nil {
		return nil
	}
	nowTime := time.Now()
	if config.IsInMaintenance(nowTime) {
		return nil
	}

	// 获取探针信息（用于发送通知）
	agent, err := s.agentRepo.FindById(ctx, agentID)
	if err != nil {
		s.logger.Error("获取探针信息失败", zap.Error(err))
		return err
	}

	now := nowTime.UnixMilli()

	// 检查 CPU 告警
	if config.Rules.CPUEnabled {
		s.checkAlert(ctx, config, &agent, "cpu", cpu, config.Rules.CPUThreshold, config.Rules.CPUDuration, now)
	}

	// 检查内存告警
	if config.Rules.MemoryEnabled {
		s.checkAlert(ctx, config, &agent, "memory", memory, config.Rules.MemoryThreshold, config.Rules.MemoryDuration, now)
	}

	// 检查磁盘告警
	if config.Rules.DiskEnabled {
		s.checkAlert(ctx, config, &agent, "disk", disk, config.Rules.DiskThreshold, config.Rules.DiskDuration, now)
	}

	// 检查网速告警
	if config.Rules.NetworkEnabled {
		s.checkAlert(ctx, config, &agent, "network", networkSpeed, config.Rules.NetworkThreshold, config.Rules.NetworkDuration, now)
	}

	return nil
}

// checkAlert 检查单个告警规则
func (s *AlertService) checkAlert(ctx context.Context, config *EffectiveAlertConfig, agent *models.Agent, alertType string, currentValue, threshold float64, duration int, now int64) {
	stateKey := fmt.Sprintf("%s:%s:%s", agent.ID, config.ConfigID, alertType)

	var shouldFire, shouldResolve bool

	// 从数据库加载状态
	state, err := s.AlertStateRepo.GetAlertState(ctx, stateKey)
	if err != nil {
		// 状态不存在，创建新状态
		state = &models.AlertState{
			ID:        stateKey,
			AgentID:   agent.ID,
			AlertType: alertType,
		}
	}

	// 按探针维度更新最新阈值/持续时间，支持配置变更
	state.AgentID = agent.ID
	state.AlertType = alertType
	state.ConfigID = config.ConfigID
	state.Threshold = threshold
	state.Duration = duration
	state.Value = currentValue
	state.LastCheckTime = now

	if currentValue >= threshold {
		if state.StartTime == 0 {
			state.StartTime = now
		}
		state.StartTime = config.ConditionStartAfterMaintenance(time.UnixMilli(now), state.StartTime)

		elapsedSeconds := (now - state.StartTime) / 1000
		if elapsedSeconds >= int64(duration) && !state.IsFiring {
			shouldFire = true
			state.IsFiring = true
		}
	} else {
		if state.IsFiring {
			shouldResolve = true
		}
		state.StartTime = 0
	}

	// 保存状态到数据库
	if err := s.AlertStateRepo.SaveAlertState(ctx, state); err != nil {
		s.logger.Error("保存告警状态失败", zap.Error(err))
		return
	}

	if shouldFire {
		s.fireAlert(ctx, config, agent, state)
	}

	if shouldResolve {
		s.resolveAlert(ctx, config, agent, state)
	}
}

// fireAlert 触发告警
func (s *AlertService) fireAlert(ctx context.Context, config *EffectiveAlertConfig, agent *models.Agent, state *models.AlertState) {
	s.logger.Info("触发告警",
		zap.String("agentId", agent.ID),
		zap.String("agentName", agent.Name),
		zap.String("alertType", state.AlertType),
		zap.Float64("value", state.Value),
		zap.Float64("threshold", state.Threshold),
	)

	now := time.Now().UnixMilli()

	// 创建告警记录
	record := &models.AlertRecord{
		AgentID:     agent.ID,
		AgentName:   agent.Name,
		AlertType:   state.AlertType,
		ConfigID:    config.ConfigID,
		ConfigName:  config.Name,
		Message:     s.buildAlertMessage(state),
		Threshold:   state.Threshold,
		ActualValue: state.Value,
		Level:       s.calculateLevel(state.Value, state.Threshold),
		Status:      "firing",
		FiredAt:     now,
		CreatedAt:   now,
	}

	err := s.AlertRecordRepo.CreateAlertRecord(ctx, record)
	if err != nil {
		s.logger.Error("创建告警记录失败", zap.Error(err))
		// 不回滚 IsFiring 状态,避免下次检查时重复触发
		// 记录创建失败不影响状态机,下次检查时会重试
		return
	}

	// 更新状态
	state.LastRecordID = record.ID
	if err := s.AlertStateRepo.SaveAlertState(ctx, state); err != nil {
		s.logger.Error("保存告警状态失败", zap.Error(err))
		return
	}

	// 发送通知 - 使用新的 context 避免父 context 取消影响通知发送
	go s.sendAlertNotification(record, agent)
}

// resolveAlert 恢复告警
func (s *AlertService) resolveAlert(ctx context.Context, config *EffectiveAlertConfig, agent *models.Agent, state *models.AlertState) {
	s.logger.Info("告警恢复",
		zap.String("agentId", agent.ID),
		zap.String("agentName", agent.Name),
		zap.String("alertType", state.AlertType),
		zap.Float64("value", state.Value),
	)

	if state.LastRecordID > 0 {
		existingRecord, err := s.AlertRecordRepo.GetAlertRecordByID(ctx, state.LastRecordID)
		if err != nil {
			s.logger.Error("获取告警记录失败", zap.Error(err))
		} else if existingRecord != nil {
			// 只有当记录状态为 firing 时才更新为 resolved
			if existingRecord.Status != "firing" {
				s.logger.Warn("告警记录状态异常,跳过恢复",
					zap.Int64("recordId", existingRecord.ID),
					zap.String("status", existingRecord.Status),
				)
			} else {
				now := time.Now().UnixMilli()
				existingRecord.Status = "resolved"
				existingRecord.ResolvedValue = state.Value
				existingRecord.ResolvedAt = now
				existingRecord.UpdatedAt = now

				err = s.AlertRecordRepo.UpdateAlertRecord(ctx, existingRecord)
				if err != nil {
					s.logger.Error("更新告警记录失败", zap.Error(err))
				} else {
					// 发送恢复通知
					go s.sendAlertNotification(existingRecord, agent)
				}
			}
		}
	}

	// 更新状态
	state.IsFiring = false
	state.LastRecordID = 0
	if err := s.AlertStateRepo.SaveAlertState(ctx, state); err != nil {
		s.logger.Error("保存告警状态失败", zap.Error(err))
	}
}

// buildAlertMessage 构建告警消息
func (s *AlertService) buildAlertMessage(state *models.AlertState) string {
	var alertTypeName string
	switch state.AlertType {
	case "cpu":
		alertTypeName = "CPU使用率"
	case "memory":
		alertTypeName = "内存使用率"
	case "disk":
		alertTypeName = "磁盘使用率"
	case "network":
		return fmt.Sprintf("网速持续%d秒超过%.2fMB/s，当前值%.2fMB/s",
			state.Duration,
			state.Threshold,
			state.Value,
		)
	case "cert":
		return fmt.Sprintf("HTTPS证书剩余天数%.0f天，低于阈值%.0f天", state.Value, state.Threshold)
	case "service":
		return fmt.Sprintf("服务持续离线%d秒", state.Duration)
	default:
		alertTypeName = state.AlertType
	}

	return fmt.Sprintf("%s持续%d秒超过%.2f%%，当前值%.2f%%",
		alertTypeName,
		state.Duration,
		state.Threshold,
		state.Value,
	)
}

// calculateLevel 计算告警级别
func (s *AlertService) calculateLevel(value, threshold float64) string {
	diff := value - threshold

	if diff < 20 {
		return "info"
	} else if diff < 50 {
		return "warning"
	} else {
		return "critical"
	}
}

// sendAlertNotification 发送告警通知(通过队列异步发送)
func (s *AlertService) sendAlertNotification(record *models.AlertRecord, agent *models.Agent) {
	s.notificationQueue.Enqueue(record.ID, agent)
}

// sendAlertNotificationSync 同步发送告警通知(供队列worker调用)
func (s *AlertService) sendAlertNotificationSync(ctx context.Context, record *models.AlertRecord, agent *models.Agent) error {
	channelConfigs, err := s.propertyService.GetNotificationChannelConfigs(ctx)
	if err != nil {
		s.logger.Error("获取通知渠道配置失败", zap.Error(err))
		return fmt.Errorf("获取通知渠道配置失败: %w", err)
	}

	var enabledChannels []models.NotificationChannelConfig
	for _, channel := range channelConfigs {
		if channel.Enabled {
			enabledChannels = append(enabledChannels, channel)
		}
	}

	// 按告警命中的规则选择推送渠道类型与 IP 打码配置（规则未指定渠道或已删除时推送所有启用渠道）
	enabledChannels, maskIP := s.resolveChannelsAndMaskIP(ctx, record, enabledChannels)

	if len(enabledChannels) == 0 {
		return fmt.Errorf("没有启用的通知渠道")
	}

	return s.notifier.SendNotificationByConfigs(ctx, enabledChannels, record, agent, maskIP)
}

// resolveChannelsAndMaskIP 根据告警记录命中的规则解析渠道选择与 IP 打码配置
func (s *AlertService) resolveChannelsAndMaskIP(ctx context.Context, record *models.AlertRecord, channels []models.NotificationChannelConfig) ([]models.NotificationChannelConfig, bool) {
	if record.ConfigID == "" {
		return channels, false
	}

	rule, err := s.alertRuleService.FindById(ctx, record.ConfigID)
	if err != nil {
		return channels, false
	}

	if len(rule.Channels) == 0 {
		return channels, rule.MaskIP
	}

	channelTypes := make(map[string]struct{}, len(rule.Channels))
	for _, t := range rule.Channels {
		channelTypes[t] = struct{}{}
	}

	filtered := make([]models.NotificationChannelConfig, 0, len(channels))
	for _, channel := range channels {
		if _, ok := channelTypes[channel.Type]; ok {
			filtered = append(filtered, channel)
		}
	}
	return filtered, rule.MaskIP
}

// CheckAgentExpireAlerts 检查机器到期提醒（按各主机命中的告警规则检查，未命中规则的主机不提醒）
func (s *AlertService) CheckAgentExpireAlerts(ctx context.Context) error {
	now := time.Now().UnixMilli()
	agents, err := s.agentRepo.FindAll(ctx)
	if err != nil {
		return err
	}

	agentIds := make([]string, 0, len(agents))
	for _, agent := range agents {
		agentIds = append(agentIds, agent.ID)
	}

	// 解析各主机生效的告警配置
	configs, err := s.alertRuleService.ResolveForAgents(ctx, agentIds)
	if err != nil {
		return err
	}

	for _, agent := range agents {
		config := configs[agent.ID]
		if config == nil {
			continue
		}
		if err := s.checkAgentExpireAlert(ctx, config, &agent, now); err != nil {
			s.logger.Error("检查机器过期提醒失败",
				zap.String("agentId", agent.ID),
				zap.Error(err),
			)
		}
	}

	return nil
}

func (s *AlertService) checkAgentExpireAlert(ctx context.Context, config *EffectiveAlertConfig, agent *models.Agent, now int64) error {
	stateKey := fmt.Sprintf("%s:%s:%s:%s", agent.ID, config.ConfigID, agentExpireAlertType, agent.ID)
	state, err := s.AlertStateRepo.GetAlertState(ctx, stateKey)
	if err != nil {
		state = &models.AlertState{
			ID:        stateKey,
			AgentID:   agent.ID,
			AlertType: agentExpireAlertType,
		}
	}

	daysLeft := agentExpireDaysLeft(agent.ExpireTime, now)
	previousReminderDay := int(state.Threshold)
	state.AgentID = agent.ID
	state.AlertType = agentExpireAlertType
	state.ConfigID = config.ConfigID
	state.Duration = 0
	state.Value = daysLeft
	state.LastCheckTime = now

	reminderDay, inReminderWindow := agentExpireReminderDay(agent.ExpireTime, now)
	expireNotificationEnabled := isNotificationEnabled(config.Notifications, NotificationTypeAgentExpire)
	if !expireNotificationEnabled || !inReminderWindow {
		shouldResolve := state.IsFiring
		state.Threshold = 0

		if err := s.AlertStateRepo.SaveAlertState(ctx, state); err != nil {
			return fmt.Errorf("保存告警状态失败: %w", err)
		}

		if shouldResolve {
			// 关闭通知开关时只清理状态，不额外发送恢复通知；续期或清空到期时间时发送恢复通知。
			s.resolveAgentExpireAlert(ctx, config, agent, state, expireNotificationEnabled)
		}
		return nil
	}

	// 首次进入提醒窗口，或从 7 天进入 3 天、再进入 1 天时，各创建一次提醒。
	// 若管理员把到期时间改到另一个提醒窗口，也用新的里程碑替换旧提醒。
	shouldFire := !state.IsFiring || previousReminderDay != reminderDay
	if state.IsFiring && previousReminderDay != reminderDay {
		s.resolveAgentExpireAlert(ctx, config, agent, state, false)
	}
	state.Threshold = float64(reminderDay)
	state.IsFiring = true

	if err := s.AlertStateRepo.SaveAlertState(ctx, state); err != nil {
		return fmt.Errorf("保存告警状态失败: %w", err)
	}

	if shouldFire {
		s.fireAgentExpireAlert(ctx, config, agent, state, now)
	}

	return nil
}

func (s *AlertService) fireAgentExpireAlert(ctx context.Context, config *EffectiveAlertConfig, agent *models.Agent, state *models.AlertState, now int64) {
	s.logger.Info("触发机器过期提醒",
		zap.String("agentId", agent.ID),
		zap.String("agentName", agent.Name),
		zap.Int64("expireTime", agent.ExpireTime),
		zap.Float64("daysLeft", state.Value),
	)

	record := &models.AlertRecord{
		AgentID:     agent.ID,
		AgentName:   agent.Name,
		AlertType:   agentExpireAlertType,
		ConfigID:    config.ConfigID,
		ConfigName:  config.Name,
		Message:     s.buildAgentExpireMessage(agent, now),
		Threshold:   state.Threshold,
		ActualValue: state.Value,
		Level:       s.calculateAgentExpireLevel(state.Value),
		Status:      "firing",
		FiredAt:     now,
		CreatedAt:   now,
	}

	if err := s.AlertRecordRepo.CreateAlertRecord(ctx, record); err != nil {
		s.logger.Error("创建机器过期提醒记录失败", zap.Error(err))
		return
	}

	state.LastRecordID = record.ID
	if err := s.AlertStateRepo.SaveAlertState(ctx, state); err != nil {
		s.logger.Error("保存告警状态失败", zap.Error(err))
		return
	}

	go s.sendAlertNotification(record, agent)
}

func (s *AlertService) resolveAgentExpireAlert(ctx context.Context, config *EffectiveAlertConfig, agent *models.Agent, state *models.AlertState, notify bool) {
	s.logger.Info("机器过期提醒恢复",
		zap.String("agentId", agent.ID),
		zap.String("agentName", agent.Name),
		zap.Int64("expireTime", agent.ExpireTime),
		zap.Float64("daysLeft", state.Value),
	)

	if state.LastRecordID > 0 {
		existingRecord, err := s.AlertRecordRepo.GetAlertRecordByID(ctx, state.LastRecordID)
		if err != nil {
			s.logger.Error("获取机器过期提醒记录失败", zap.Error(err))
		} else if existingRecord != nil && existingRecord.Status == "firing" {
			now := time.Now().UnixMilli()
			existingRecord.Status = "resolved"
			existingRecord.ResolvedValue = state.Value
			existingRecord.ResolvedAt = now
			existingRecord.UpdatedAt = now

			if err := s.AlertRecordRepo.UpdateAlertRecord(ctx, existingRecord); err != nil {
				s.logger.Error("更新机器过期提醒记录失败", zap.Error(err))
			} else if notify {
				go s.sendAlertNotification(existingRecord, agent)
			}
		}
	}

	state.IsFiring = false
	state.LastRecordID = 0
	if err := s.AlertStateRepo.SaveAlertState(ctx, state); err != nil {
		s.logger.Error("保存告警状态失败", zap.Error(err))
	}
}

func (s *AlertService) buildAgentExpireMessage(agent *models.Agent, now int64) string {
	agentName := agent.Name
	if agentName == "" {
		agentName = agent.Hostname
	}
	if agentName == "" {
		agentName = agent.ID
	}

	expireAt := time.UnixMilli(agent.ExpireTime).Format("2006-01-02 15:04:05")
	if agent.ExpireTime <= now {
		return fmt.Sprintf("机器 %s 已于 %s 过期", agentName, expireAt)
	}

	remaining := agent.ExpireTime - now
	dayMs := int64(24 * time.Hour / time.Millisecond)
	if remaining < dayMs {
		return fmt.Sprintf("机器 %s 将于 %s 过期，剩余不足1天", agentName, expireAt)
	}

	days := remaining / dayMs
	if remaining%dayMs != 0 {
		days++
	}
	return fmt.Sprintf("机器 %s 将于 %s 过期，剩余%d天", agentName, expireAt, days)
}

func (s *AlertService) calculateAgentExpireLevel(daysLeft float64) string {
	if daysLeft <= 0 {
		return "critical"
	}
	if daysLeft <= 3 {
		return "warning"
	}
	return "info"
}

func agentExpireDaysLeft(expireTime int64, now int64) float64 {
	if expireTime <= 0 {
		return 0
	}
	return float64(expireTime-now) / float64(24*time.Hour/time.Millisecond)
}

// agentExpireReminderDay 返回当前应命中的提醒里程碑。若首次配置时已不足 3 天或 1 天，
// 只命中最近的一个里程碑，避免在同一次检查中连续发送多条历史提醒。
func agentExpireReminderDay(expireTime int64, now int64) (int, bool) {
	if expireTime <= 0 {
		return 0, false
	}

	remaining := expireTime - now
	dayMs := int64(24 * time.Hour / time.Millisecond)
	for i := len(agentExpireReminderDays) - 1; i >= 0; i-- {
		reminderDay := agentExpireReminderDays[i]
		if remaining <= int64(reminderDay)*dayMs {
			return reminderDay, true
		}
	}
	return 0, false
}

// CheckMonitorAlerts 检查监控相关告警（证书、服务下线、探针离线，按各主机生效的规则检查）
func (s *AlertService) CheckMonitorAlerts(ctx context.Context) error {
	now := time.Now().UnixMilli()

	// 检查证书告警
	if err := s.checkCertificateAlerts(ctx, now); err != nil {
		s.logger.Error("检查证书告警失败", zap.Error(err))
	}

	// 检查服务下线告警
	if err := s.checkServiceDownAlerts(ctx, now); err != nil {
		s.logger.Error("检查服务下线告警失败", zap.Error(err))
	}

	// 检查探针离线告警
	if err := s.checkAgentOfflineAlerts(ctx, now); err != nil {
		s.logger.Error("检查探针离线告警失败", zap.Error(err))
	}

	return nil
}

// checkCertificateAlerts 检查证书告警
func (s *AlertService) checkCertificateAlerts(ctx context.Context, now int64) error {
	// 获取所有最新的监控指标（仅HTTPS类型）
	// 这里需要查询最新的 monitor_metrics 记录，获取证书剩余天数
	monitors, err := s.monitorService.GetLatestMonitorMetricsByType(ctx, "http")
	if err != nil {
		return err
	}

	// 收集所有 agentIds，批量查询探针信息
	agentIdSet := make(map[string]bool)
	for _, monitor := range monitors {
		if monitor.AgentId != "" {
			agentIdSet[monitor.AgentId] = true
		}
	}

	agentIds := make([]string, 0, len(agentIdSet))
	for id := range agentIdSet {
		agentIds = append(agentIds, id)
	}

	agentMap := make(map[string]*models.Agent)
	if len(agentIds) > 0 {
		agents, err := s.agentRepo.ListByIDs(ctx, agentIds)
		if err != nil {
			s.logger.Error("批量获取探针信息失败", zap.Error(err))
			return err
		}
		for i := range agents {
			agentMap[agents[i].ID] = &agents[i]
		}
	}

	// 解析各主机生效的告警配置
	configs, err := s.alertRuleService.ResolveForAgents(ctx, agentIds)
	if err != nil {
		return err
	}

	for _, monitor := range monitors {
		// 如果证书不存在或已过期，跳过
		if monitor.CertExpiryTime == 0 {
			continue
		}

		config := configs[monitor.AgentId]
		if config == nil || !config.Rules.CertEnabled {
			continue
		}
		if config.IsInMaintenance(time.UnixMilli(now)) {
			continue
		}

		certDaysLeft := float64(monitor.CertDaysLeft)

		// 从 map 中获取探针信息
		agent, exists := agentMap[monitor.AgentId]
		if !exists {
			s.logger.Error("探针信息不存在", zap.String("agentId", monitor.AgentId))
			continue
		}

		// 检查证书剩余天数是否低于阈值
		if certDaysLeft <= config.Rules.CertThreshold && certDaysLeft >= 0 {
			// 触发告警（证书告警不需要持续时间，直接触发）
			s.checkCertAlert(ctx, config, agent, &monitor, certDaysLeft, now)
		} else {
			// 恢复告警（如果之前触发过）
			s.resolveCertAlert(ctx, config, agent, &monitor, certDaysLeft)
		}
	}

	return nil
}

// checkCertAlert 检查并触发证书告警
func (s *AlertService) checkCertAlert(ctx context.Context, config *EffectiveAlertConfig, agent *models.Agent, monitor *protocol.MonitorData, certDaysLeft float64, now int64) {
	stateKey := fmt.Sprintf("%s:%s:cert:%s", agent.ID, config.ConfigID, monitor.MonitorId)

	// 从数据库加载状态
	state, err := s.AlertStateRepo.GetAlertState(ctx, stateKey)
	if err != nil {
		// 状态不存在，创建新状态
		state = &models.AlertState{
			ID:        stateKey,
			AgentID:   agent.ID,
			AlertType: "cert",
		}
	}
	state.AgentID = agent.ID
	state.AlertType = "cert"
	state.ConfigID = config.ConfigID
	state.Threshold = config.Rules.CertThreshold
	state.Duration = 0
	state.Value = certDaysLeft
	state.LastCheckTime = now

	shouldFire := certDaysLeft <= config.Rules.CertThreshold && !state.IsFiring

	if shouldFire {
		state.IsFiring = true
	}

	// 保存状态到数据库
	if err := s.AlertStateRepo.SaveAlertState(ctx, state); err != nil {
		s.logger.Error("保存告警状态失败", zap.Error(err))
		return
	}

	if !shouldFire {
		return
	}

	s.logger.Info("触发证书告警",
		zap.String("agentId", agent.ID),
		zap.String("monitorId", monitor.MonitorId),
		zap.String("monitorName", monitor.MonitorName),
		zap.String("target", monitor.Target),
		zap.Float64("certDaysLeft", certDaysLeft),
		zap.Float64("threshold", config.Rules.CertThreshold),
	)

	// 构建告警消息，优先使用监控任务名称
	var message string
	if monitor.MonitorName != "" {
		message = fmt.Sprintf("监控项 %s (%s) 的HTTPS证书剩余天数%.0f天，低于阈值%.0f天", monitor.MonitorName, monitor.Target, certDaysLeft, config.Rules.CertThreshold)
	} else {
		message = fmt.Sprintf("监控项 %s 的HTTPS证书剩余天数%.0f天，低于阈值%.0f天", monitor.Target, certDaysLeft, config.Rules.CertThreshold)
	}

	record := &models.AlertRecord{
		AgentID:     agent.ID,
		AgentName:   agent.Name,
		AlertType:   "cert",
		ConfigID:    config.ConfigID,
		ConfigName:  config.Name,
		Message:     message,
		Threshold:   config.Rules.CertThreshold,
		ActualValue: certDaysLeft,
		Level:       s.calculateCertLevel(certDaysLeft),
		Status:      "firing",
		FiredAt:     now,
		CreatedAt:   now,
	}

	err = s.AlertRecordRepo.CreateAlertRecord(ctx, record)
	if err != nil {
		s.logger.Error("创建证书告警记录失败", zap.Error(err))
		return
	}

	state.LastRecordID = record.ID
	if err := s.AlertStateRepo.SaveAlertState(ctx, state); err != nil {
		s.logger.Error("保存告警状态失败", zap.Error(err))
		return
	}

	// 发送通知
	go s.sendAlertNotification(record, agent)
}

// resolveCertAlert 恢复证书告警
func (s *AlertService) resolveCertAlert(ctx context.Context, config *EffectiveAlertConfig, agent *models.Agent, monitor *protocol.MonitorData, certDaysLeft float64) {
	stateKey := fmt.Sprintf("%s:%s:cert:%s", agent.ID, config.ConfigID, monitor.MonitorId)

	state, err := s.AlertStateRepo.GetAlertState(ctx, stateKey)
	if err != nil || !state.IsFiring {
		return
	}

	s.logger.Info("证书告警恢复",
		zap.String("agentId", agent.ID),
		zap.String("monitorId", monitor.MonitorId),
		zap.String("target", monitor.Target),
		zap.Float64("certDaysLeft", certDaysLeft),
	)

	if state.LastRecordID > 0 {
		existingRecord, err := s.AlertRecordRepo.GetAlertRecordByID(ctx, state.LastRecordID)
		if err != nil {
			s.logger.Error("获取证书告警记录失败", zap.Error(err))
		} else if existingRecord != nil && existingRecord.Status == "firing" {
			now := time.Now().UnixMilli()
			existingRecord.Status = "resolved"
			existingRecord.ResolvedValue = certDaysLeft
			existingRecord.ResolvedAt = now
			existingRecord.UpdatedAt = now

			err = s.AlertRecordRepo.UpdateAlertRecord(ctx, existingRecord)
			if err != nil {
				s.logger.Error("更新证书告警记录失败", zap.Error(err))
			} else {
				// 发送恢复通知
				go s.sendAlertNotification(existingRecord, agent)
			}
		}
	}

	state.IsFiring = false
	state.LastRecordID = 0
	if err := s.AlertStateRepo.SaveAlertState(ctx, state); err != nil {
		s.logger.Error("保存告警状态失败", zap.Error(err))
		return
	}
}

// calculateCertLevel 计算证书告警级别
func (s *AlertService) calculateCertLevel(daysLeft float64) string {
	if daysLeft <= 7 {
		return "critical"
	} else if daysLeft <= 30 {
		return "warning"
	} else {
		return "info"
	}
}

// checkServiceDownAlerts 检查服务下线告警
func (s *AlertService) checkServiceDownAlerts(ctx context.Context, now int64) error {
	// 获取所有最新的监控指标
	monitors, err := s.monitorService.GetAllLatestMonitorMetrics(ctx)
	if err != nil {
		return err
	}

	// 收集所有 agentIds，批量查询探针信息
	agentIdSet := make(map[string]bool)
	for _, monitor := range monitors {
		if monitor.AgentId != "" {
			agentIdSet[monitor.AgentId] = true
		}
	}

	agentIds := make([]string, 0, len(agentIdSet))
	for id := range agentIdSet {
		agentIds = append(agentIds, id)
	}

	agentMap := make(map[string]*models.Agent)
	if len(agentIds) > 0 {
		agents, err := s.agentRepo.ListByIDs(ctx, agentIds)
		if err != nil {
			s.logger.Error("批量获取探针信息失败", zap.Error(err))
			return err
		}
		for i := range agents {
			agentMap[agents[i].ID] = &agents[i]
		}
	}

	// 解析各主机生效的告警配置
	configs, err := s.alertRuleService.ResolveForAgents(ctx, agentIds)
	if err != nil {
		return err
	}

	for _, monitor := range monitors {
		// 从 map 中获取探针信息
		agent, exists := agentMap[monitor.AgentId]
		if !exists {
			s.logger.Error("探针信息不存在", zap.String("agentId", monitor.AgentId))
			continue
		}

		config := configs[monitor.AgentId]
		if config == nil || !config.Rules.ServiceEnabled {
			continue
		}
		if config.IsInMaintenance(time.UnixMilli(now)) {
			continue
		}

		stateKey := fmt.Sprintf("%s:%s:service:%s", agent.ID, config.ConfigID, monitor.MonitorId)

		var shouldFire, shouldResolve bool

		// 从数据库加载状态
		state, err := s.AlertStateRepo.GetAlertState(ctx, stateKey)
		if err != nil {
			// 状态不存在，创建新状态
			state = &models.AlertState{
				ID:        stateKey,
				AgentID:   agent.ID,
				AlertType: "service",
			}
		}
		state.AgentID = agent.ID
		state.AlertType = "service"
		state.ConfigID = config.ConfigID
		state.Duration = config.Rules.ServiceDuration
		state.LastCheckTime = now

		if monitor.Status == "down" {
			if state.StartTime == 0 {
				state.StartTime = monitor.CheckedAt
			}
			state.StartTime = config.ConditionStartAfterMaintenance(time.UnixMilli(now), state.StartTime)

			elapsedSeconds := (now - state.StartTime) / 1000
			if elapsedSeconds >= int64(config.Rules.ServiceDuration) && !state.IsFiring {
				shouldFire = true
				state.IsFiring = true
			}
		} else {
			if state.IsFiring {
				shouldResolve = true
			}
			state.StartTime = 0
		}

		// 保存状态到数据库
		if err := s.AlertStateRepo.SaveAlertState(ctx, state); err != nil {
			s.logger.Error("保存告警状态失败", zap.Error(err))
			continue
		}

		if shouldFire {
			s.fireServiceDownAlert(ctx, config, agent, &monitor, state, now)
		}

		if shouldResolve {
			s.resolveServiceDownAlert(ctx, config, agent, &monitor, state)
		}
	}

	return nil
}

// fireServiceDownAlert 触发服务下线告警
func (s *AlertService) fireServiceDownAlert(ctx context.Context, config *EffectiveAlertConfig, agent *models.Agent, monitor *protocol.MonitorData, state *models.AlertState, now int64) {
	s.logger.Info("触发服务下线告警",
		zap.String("agentId", agent.ID),
		zap.String("monitorId", monitor.MonitorId),
		zap.String("monitorName", monitor.MonitorName),
		zap.String("target", monitor.Target),
		zap.Int("duration", state.Duration),
	)

	// 构建告警消息，优先使用监控任务名称
	var message string
	if monitor.MonitorName != "" {
		message = fmt.Sprintf("监控项 %s (%s) 持续离线%d秒", monitor.MonitorName, monitor.Target, state.Duration)
	} else {
		message = fmt.Sprintf("监控项 %s 持续离线%d秒", monitor.Target, state.Duration)
	}

	// 创建告警记录
	record := &models.AlertRecord{
		AgentID:     agent.ID,
		AgentName:   agent.Name,
		AlertType:   "service",
		ConfigID:    config.ConfigID,
		ConfigName:  config.Name,
		Message:     message,
		Threshold:   0,
		ActualValue: float64(state.Duration),
		Level:       "critical",
		Status:      "firing",
		FiredAt:     now,
		CreatedAt:   now,
	}

	err := s.AlertRecordRepo.CreateAlertRecord(ctx, record)
	if err != nil {
		s.logger.Error("创建服务下线告警记录失败", zap.Error(err))
		return
	}

	state.LastRecordID = record.ID
	if err := s.AlertStateRepo.SaveAlertState(ctx, state); err != nil {
		s.logger.Error("保存告警状态失败", zap.Error(err))
		return
	}

	// 发送通知
	go s.sendAlertNotification(record, agent)
}

// resolveServiceDownAlert 恢复服务下线告警
func (s *AlertService) resolveServiceDownAlert(ctx context.Context, config *EffectiveAlertConfig, agent *models.Agent, monitor *protocol.MonitorData, state *models.AlertState) {
	s.logger.Info("服务下线告警恢复",
		zap.String("agentId", agent.ID),
		zap.String("monitorId", monitor.MonitorId),
		zap.String("target", monitor.Target),
	)

	if state.LastRecordID > 0 {
		existingRecord, err := s.AlertRecordRepo.GetAlertRecordByID(ctx, state.LastRecordID)
		if err != nil {
			s.logger.Error("获取服务下线告警记录失败", zap.Error(err))
		} else if existingRecord != nil && existingRecord.Status == "firing" {
			now := time.Now().UnixMilli()
			existingRecord.Status = "resolved"
			existingRecord.ResolvedValue = 0 // 服务已恢复在线
			existingRecord.ResolvedAt = now
			existingRecord.UpdatedAt = now

			err = s.AlertRecordRepo.UpdateAlertRecord(ctx, existingRecord)
			if err != nil {
				s.logger.Error("更新服务下线告警记录失败", zap.Error(err))
			} else {
				// 发送恢复通知
				go s.sendAlertNotification(existingRecord, agent)
			}
		}
	}

	state.IsFiring = false
	state.LastRecordID = 0
	if err := s.AlertStateRepo.SaveAlertState(ctx, state); err != nil {
		s.logger.Error("保存告警状态失败", zap.Error(err))
		return
	}
}

// checkAgentOfflineAlerts 检查探针离线告警
func (s *AlertService) checkAgentOfflineAlerts(ctx context.Context, now int64) error {
	// 获取所有探针
	agents, err := s.agentRepo.FindAll(ctx)
	if err != nil {
		return err
	}

	agentIds := make([]string, 0, len(agents))
	for _, agent := range agents {
		agentIds = append(agentIds, agent.ID)
	}

	// 解析各探针生效的告警配置
	configs, err := s.alertRuleService.ResolveForAgents(ctx, agentIds)
	if err != nil {
		return err
	}

	for _, agent := range agents {
		config := configs[agent.ID]
		if config == nil || !config.Rules.AgentOfflineEnabled {
			continue
		}
		if config.IsInMaintenance(time.UnixMilli(now)) {
			continue
		}

		stateKey := fmt.Sprintf("%s:%s:agent_offline:%s", agent.ID, config.ConfigID, agent.ID)

		// status 由 WebSocket 连接的注册/注销同步维护，是当前是否离线的
		// 权威状态；last_seen_at 仅用于在已经离线时计算持续时间。在线连接
		// 的 last_seen_at 会按间隔去抖写库，不能单独据此判定离线，否则
		// 写库间隔与告警阈值相同时会在边界上反复误报和恢复。
		isOffline, offlineStartTime, offlineSeconds := agentOfflineCondition(config, &agent, now)

		// 从数据库加载状态
		state, err := s.AlertStateRepo.GetAlertState(ctx, stateKey)
		if err != nil {
			// 状态不存在，创建新状态
			state = &models.AlertState{
				ID:        stateKey,
				AgentID:   agent.ID,
				AlertType: "agent_offline",
			}
		}

		state.AgentID = agent.ID
		state.AlertType = "agent_offline"
		state.ConfigID = config.ConfigID
		state.Duration = config.Rules.AgentOfflineDuration
		state.Threshold = float64(config.Rules.AgentOfflineDuration)
		state.Value = float64(offlineSeconds)
		state.StartTime = offlineStartTime
		state.LastCheckTime = now

		var shouldFire, shouldResolve bool

		if state.IsFiring {
			// 已有告警只在 WebSocket 连接实际恢复后解除，不能因为维护结束后
			// 重新累计或 last_seen_at 刷新而伪造恢复。
			shouldResolve = !isOffline
		} else if isOffline && offlineSeconds >= int64(config.Rules.AgentOfflineDuration) {
			shouldFire = true
			state.IsFiring = true
		}

		// 保存状态到数据库
		if err := s.AlertStateRepo.SaveAlertState(ctx, state); err != nil {
			s.logger.Error("保存告警状态失败", zap.Error(err))
			continue
		}

		if shouldFire {
			s.fireAgentOfflineAlert(ctx, config, &agent, state, offlineSeconds, now)
		}

		if shouldResolve {
			s.resolveAgentOfflineAlert(ctx, config, &agent, state)
		}
	}

	return nil
}

// agentOfflineCondition 返回探针当前是否离线，以及扣除维护窗口后的离线
// 起点和持续秒数。在线状态下时间统一归零，避免去抖写入的 last_seen_at
// 被误当作连接活性判断。
func agentOfflineCondition(config *EffectiveAlertConfig, agent *models.Agent, now int64) (bool, int64, int64) {
	if agent.Status == 1 {
		return false, 0, 0
	}

	offlineStartTime := config.ConditionStartAfterMaintenance(time.UnixMilli(now), agent.LastSeenAt)
	offlineSeconds := int64(0)
	if now > offlineStartTime {
		offlineSeconds = (now - offlineStartTime) / 1000
	}
	return true, offlineStartTime, offlineSeconds
}

// fireAgentOfflineAlert 触发探针离线告警
func (s *AlertService) fireAgentOfflineAlert(ctx context.Context, config *EffectiveAlertConfig, agent *models.Agent, state *models.AlertState, offlineSeconds int64, now int64) {
	s.logger.Info("触发探针离线告警",
		zap.String("agentId", agent.ID),
		zap.String("agentName", agent.Name),
		zap.Int64("offlineSeconds", offlineSeconds),
		zap.Int("threshold", state.Duration),
	)

	// 创建告警记录
	record := &models.AlertRecord{
		AgentID:     agent.ID,
		AgentName:   agent.Name,
		AlertType:   "agent_offline",
		ConfigID:    config.ConfigID,
		ConfigName:  config.Name,
		Message:     fmt.Sprintf("探针 %s 已离线%d秒，超过阈值%d秒", agent.Name, offlineSeconds, state.Duration),
		Threshold:   float64(state.Duration),
		ActualValue: float64(offlineSeconds),
		Level:       "critical",
		Status:      "firing",
		FiredAt:     now,
		CreatedAt:   now,
	}

	err := s.AlertRecordRepo.CreateAlertRecord(ctx, record)
	if err != nil {
		s.logger.Error("创建探针离线告警记录失败", zap.Error(err))
		return
	}

	state.LastRecordID = record.ID
	if err := s.AlertStateRepo.SaveAlertState(ctx, state); err != nil {
		s.logger.Error("保存告警状态失败", zap.Error(err))
		return
	}

	// 发送通知
	go s.sendAlertNotification(record, agent)
}

// resolveAgentOfflineAlert 恢复探针离线告警
func (s *AlertService) resolveAgentOfflineAlert(ctx context.Context, config *EffectiveAlertConfig, agent *models.Agent, state *models.AlertState) {
	s.logger.Info("探针离线告警恢复",
		zap.String("agentId", agent.ID),
		zap.String("agentName", agent.Name),
	)

	// 更新告警记录状态
	if state.LastRecordID > 0 {
		existingRecord, err := s.AlertRecordRepo.GetAlertRecordByID(ctx, state.LastRecordID)
		if err != nil {
			s.logger.Error("获取探针离线告警记录失败", zap.Error(err))
		} else if existingRecord != nil && existingRecord.Status == "firing" {
			now := time.Now().UnixMilli()
			existingRecord.Status = "resolved"
			existingRecord.ResolvedValue = 0 // 探针已恢复在线
			existingRecord.ResolvedAt = now
			existingRecord.UpdatedAt = now

			err = s.AlertRecordRepo.UpdateAlertRecord(ctx, existingRecord)
			if err != nil {
				s.logger.Error("更新探针离线告警记录失败", zap.Error(err))
			} else {
				// 发送恢复通知
				go s.sendAlertNotification(existingRecord, agent)
			}
		}
	}

	state.IsFiring = false
	state.LastRecordID = 0
	if err := s.AlertStateRepo.SaveAlertState(ctx, state); err != nil {
		s.logger.Error("保存告警状态失败", zap.Error(err))
	}
}
