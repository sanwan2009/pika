package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/dushixiang/pika/internal/models"
	"github.com/dushixiang/pika/internal/protocol"
	"github.com/dushixiang/pika/internal/repo"
	"github.com/go-orz/orz"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type AgentService struct {
	logger *zap.Logger
	*orz.Service
	AgentRepo        *repo.AgentRepo
	monitorStatsRepo *repo.MonitorStatsRepo
	apiKeyService    *ApiKeyService
	metricService    *MetricService
}

func NewAgentService(logger *zap.Logger, db *gorm.DB, apiKeyService *ApiKeyService, metricService *MetricService) *AgentService {
	return &AgentService{
		logger:           logger,
		Service:          orz.NewService(db),
		AgentRepo:        repo.NewAgentRepo(db),
		monitorStatsRepo: repo.NewMonitorStatsRepo(db),
		apiKeyService:    apiKeyService,
		metricService:    metricService,
	}
}

// RegisterAgent 注册探针
func (s *AgentService) RegisterAgent(ctx context.Context, ip string, info *protocol.AgentInfo, apiKey string) (*models.Agent, error) {
	// 验证API密钥
	if _, err := s.apiKeyService.ValidateApiKey(ctx, apiKey); err != nil {
		s.logger.Warn("agent registration failed: invalid api key",
			zap.String("agentID", info.ID),
			zap.String("hostname", info.Hostname),
		)
		return nil, err
	}

	// 验证探针 ID
	if info.ID == "" {
		return nil, fmt.Errorf("agent ID 不能为空")
	}

	// 使用探针的持久化 ID 来识别同一个探针
	// 这样即使主机名或 IP 变化，也能正确识别
	existingAgent, err := s.AgentRepo.FindById(ctx, info.ID)
	if err == nil {
		// 更新现有探针信息（允许主机名、IP、名称等变化）
		now := time.Now().UnixMilli()
		existingAgent.Hostname = info.Hostname
		existingAgent.IP = ip
		existingAgent.OS = info.OS
		existingAgent.Arch = info.Arch
		existingAgent.Version = info.Version
		existingAgent.Status = 1
		existingAgent.LastSeenAt = now
		existingAgent.UpdatedAt = now

		if err := s.AgentRepo.UpdateById(ctx, &existingAgent); err != nil {
			return nil, err
		}
		s.logger.Info("agent re-registered",
			zap.String("agentID", existingAgent.ID),
			zap.String("name", info.Name),
			zap.String("hostname", info.Hostname),
			zap.String("ip", ip),
			zap.String("version", info.Version))
		return &existingAgent, nil
	}

	// 创建新探针（使用客户端提供的持久化 ID）
	now := time.Now().UnixMilli()
	agent := &models.Agent{
		ID:         info.ID, // 使用客户端持久化的 ID
		Name:       info.Name,
		Hostname:   info.Hostname,
		IP:         ip,
		OS:         info.OS,
		Arch:       info.Arch,
		Version:    info.Version,
		Status:     1,
		LastSeenAt: now,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if err := s.AgentRepo.Create(ctx, agent); err != nil {
		return nil, err
	}

	s.logger.Info("agent registered successfully",
		zap.String("agentID", agent.ID),
		zap.String("name", info.Name),
		zap.String("hostname", info.Hostname),
		zap.String("ip", ip),
		zap.String("version", info.Version))
	return agent, nil
}

// UpdateAgentStatus 更新探针状态
func (s *AgentService) UpdateAgentStatus(ctx context.Context, agentID string, status int) error {
	return s.AgentRepo.UpdateStatus(ctx, agentID, status, time.Now().UnixMilli())
}

// GetAgent 获取探针信息
func (s *AgentService) GetAgent(ctx context.Context, agentID string) (*models.Agent, error) {
	agent, err := s.AgentRepo.FindById(ctx, agentID)
	if err != nil {
		return nil, err
	}
	return &agent, nil
}

// ListAgents 列出所有探针
func (s *AgentService) ListAgents(ctx context.Context) ([]models.Agent, error) {
	return s.AgentRepo.FindAll(ctx)
}

// ListOnlineAgents 列出所有在线探针
func (s *AgentService) ListOnlineAgents(ctx context.Context) ([]models.Agent, error) {
	return s.AgentRepo.FindOnlineAgents(ctx)
}

// HandleMetricData 处理指标数据
func (s *AgentService) HandleMetricData(ctx context.Context, agentID string, metricType string, data json.RawMessage) error {
	return s.metricService.HandleMetricData(ctx, agentID, metricType, data)
}

// GetMetrics 获取聚合指标数据（自动路由到聚合表或原始表）
func (s *AgentService) GetMetrics(ctx context.Context, agentID, metricType string, start, end int64, interval int, interfaceName string) (interface{}, error) {
	return s.metricService.GetMetrics(ctx, agentID, metricType, start, end, interval, interfaceName)
}

// GetAvailableNetworkInterfaces 获取探针的可用网卡列表
func (s *AgentService) GetAvailableNetworkInterfaces(ctx context.Context, agentID string) ([]string, error) {
	return s.metricService.GetAvailableNetworkInterfaces(ctx, agentID)
}

// GetLatestMetrics 获取最新指标
func (s *AgentService) GetLatestMetrics(ctx context.Context, agentID string) (*LatestMetrics, error) {
	return s.metricService.GetLatestMetrics(ctx, agentID)
}

// StartCleanupTask 启动数据清理任务
func (s *AgentService) StartCleanupTask(ctx context.Context) {
	s.metricService.StartCleanupTask(ctx)
}

// StartAggregationTask 启动聚合下采样任务
func (s *AgentService) StartAggregationTask(ctx context.Context) {
	s.metricService.StartAggregationTask(ctx)
}

// HandleCommandResponse 处理指令响应
func (s *AgentService) HandleCommandResponse(ctx context.Context, agentID string, resp *protocol.CommandResponse) error {
	s.logger.Info("command response received",
		zap.String("agentID", agentID),
		zap.String("cmdID", resp.ID),
		zap.String("type", resp.Type),
		zap.String("status", resp.Status))

	// 根据指令类型处理响应
	switch resp.Type {
	case "vps_audit":
		return s.handleVPSAuditResponse(ctx, agentID, resp)
	default:
		s.logger.Warn("unknown command type", zap.String("type", resp.Type))
		return nil
	}
}

// handleVPSAuditResponse 处理VPS审计响应
func (s *AgentService) handleVPSAuditResponse(ctx context.Context, agentID string, resp *protocol.CommandResponse) error {
	if resp.Status == "error" {
		s.logger.Error("vps audit failed",
			zap.String("agentID", agentID),
			zap.String("error", resp.Error))
		return nil
	}

	if resp.Status == "running" {
		s.logger.Info("vps audit is running", zap.String("agentID", agentID))
		return nil
	}

	if resp.Status == "success" {
		// 解析审计结果
		var auditResult protocol.VPSAuditResult
		if err := json.Unmarshal([]byte(resp.Result), &auditResult); err != nil {
			s.logger.Error("failed to parse audit result", zap.Error(err))
			return err
		}

		// 存储审计结果
		return s.SaveAuditResult(ctx, agentID, &auditResult)
	}

	return nil
}

// SaveAuditResult 保存审计结果
func (s *AgentService) SaveAuditResult(ctx context.Context, agentID string, result *protocol.VPSAuditResult) error {
	// 将结果序列化为JSON存储
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return err
	}

	auditRecord := &models.AuditResult{
		AgentID:   agentID,
		Type:      "vps_audit",
		Result:    string(resultJSON),
		StartTime: result.StartTime,
		EndTime:   result.EndTime,
		CreatedAt: time.Now().UnixMilli(),
	}

	// 保存到数据库
	if err := s.AgentRepo.SaveAuditResult(ctx, auditRecord); err != nil {
		return err
	}

	s.logger.Info("审计结果保存成功",
		zap.String("agentId", agentID),
		zap.Int64("auditId", auditRecord.ID),
	)

	return nil
}

// GetAuditResult 获取最新的审计结果(原始数据)
func (s *AgentService) GetAuditResult(ctx context.Context, agentID string) (*protocol.VPSAuditResult, error) {
	record, err := s.AgentRepo.GetLatestAuditResultByType(ctx, agentID, "vps_audit")
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	var result protocol.VPSAuditResult
	if err := json.Unmarshal([]byte(record.Result), &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// ListAuditResults 获取审计结果列表
func (s *AgentService) ListAuditResults(ctx context.Context, agentID string) ([]map[string]interface{}, error) {
	records, err := s.AgentRepo.ListAuditResults(ctx, agentID)
	if err != nil {
		return nil, err
	}

	results := make([]map[string]interface{}, 0, len(records))
	for _, record := range records {
		var auditResult protocol.VPSAuditResult
		if err := json.Unmarshal([]byte(record.Result), &auditResult); err != nil {
			s.logger.Error("failed to parse audit result", zap.Error(err))
			continue
		}

		// TODO: 统计安全检查结果应该来自 Server 端分析后的 VPSAuditAnalysis
		// Agent 端已经不再产生 SecurityChecks,需要实现 Server 端分析逻辑

		results = append(results, map[string]interface{}{
			"id":          record.ID,
			"agentId":     record.AgentID,
			"type":        record.Type,
			"startTime":   record.StartTime,
			"endTime":     record.EndTime,
			"createdAt":   record.CreatedAt,
			"systemInfo":  auditResult.SystemInfo,
			"statistics":  auditResult.Statistics,
			"collectTime": auditResult.EndTime - auditResult.StartTime,
		})
	}

	return results, nil
}

// GetStatistics 获取探针统计数据
func (s *AgentService) GetStatistics(ctx context.Context) (map[string]interface{}, error) {
	total, online, err := s.AgentRepo.GetStatistics(ctx)
	if err != nil {
		return nil, err
	}

	offline := total - online
	onlineRate := 0.0
	if total > 0 {
		onlineRate = float64(online) / float64(total) * 100
	}

	return map[string]interface{}{
		"total":      total,
		"online":     online,
		"offline":    offline,
		"onlineRate": onlineRate,
	}, nil
}

// GetLatestMonitorMetrics 获取最新的监控指标
func (s *AgentService) GetLatestMonitorMetrics(ctx context.Context, agentID string) ([]models.MonitorMetric, error) {
	return s.metricService.GetLatestMonitorMetrics(ctx, agentID)
}

// GetMonitorMetrics 获取监控指标历史数据
func (s *AgentService) GetMonitorMetrics(ctx context.Context, agentID, monitorName string, start, end int64) ([]models.MonitorMetric, error) {
	return s.metricService.GetMonitorMetrics(ctx, agentID, monitorName, start, end)
}

// GetMonitorMetricsByName 获取指定监控项的历史数据
func (s *AgentService) GetMonitorMetricsByName(ctx context.Context, agentID, monitorName string, start, end int64, limit int) ([]models.MonitorMetric, error) {
	return s.metricService.GetMonitorMetricsByName(ctx, agentID, monitorName, start, end, limit)
}

// DeleteAgent 删除探针及其所有相关数据
func (s *AgentService) DeleteAgent(ctx context.Context, agentID string) error {
	// 在事务中执行所有删除操作
	return s.Transaction(ctx, func(ctx context.Context) error {
		// 1. 删除探针的所有指标数据
		if err := s.metricService.DeleteAgentMetrics(ctx, agentID); err != nil {
			s.logger.Error("删除探针指标数据失败", zap.String("agentId", agentID), zap.Error(err))
			return err
		}

		// 2. 删除探针的监控统计数据
		if err := s.monitorStatsRepo.DeleteByAgentId(ctx, agentID); err != nil {
			s.logger.Error("删除探针监控统计数据失败", zap.String("agentId", agentID), zap.Error(err))
			return err
		}

		// 3. 删除探针的审计结果
		if err := s.AgentRepo.DeleteAuditResults(ctx, agentID); err != nil {
			s.logger.Error("删除探针审计结果失败", zap.String("agentId", agentID), zap.Error(err))
			return err
		}

		// 4. 最后删除探针本身
		if err := s.AgentRepo.DeleteById(ctx, agentID); err != nil {
			s.logger.Error("删除探针失败", zap.String("agentId", agentID), zap.Error(err))
			return err
		}

		s.logger.Info("探针删除成功", zap.String("agentId", agentID))
		return nil
	})
}

// ListByAuth 根据认证状态列出探针（已登录返回全部，未登录返回公开可见）
func (s *AgentService) ListByAuth(ctx context.Context, isAuthenticated bool) ([]models.Agent, error) {
	if isAuthenticated {
		return s.AgentRepo.FindAll(ctx)
	}
	return s.AgentRepo.FindPublicAgents(ctx)
}

// GetAgentByAuth 根据认证状态获取探针（已登录返回全部，未登录返回公开可见）
func (s *AgentService) GetAgentByAuth(ctx context.Context, id string, isAuthenticated bool) (*models.Agent, error) {
	if isAuthenticated {
		agent, err := s.AgentRepo.FindById(ctx, id)
		if err != nil {
			return nil, err
		}
		return &agent, nil
	}
	return s.AgentRepo.FindPublicAgentByID(ctx, id)
}

// GetAllTags 获取所有探针的标签
func (s *AgentService) GetAllTags(ctx context.Context) ([]string, error) {
	return s.AgentRepo.GetAllTags(ctx)
}

func (s *AgentService) InitStatus(ctx context.Context) error {
	agents, err := s.AgentRepo.FindAll(ctx)
	if err != nil {
		return err
	}
	for _, agent := range agents {
		if err := s.AgentRepo.UpdateStatus(ctx, agent.ID, 0, 0); err != nil {
			return err
		}
	}
	return nil
}
