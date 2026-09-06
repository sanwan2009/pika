package service

import (
	"context"

	"github.com/pika-monitor/pika/internal/models"
	"github.com/pika-monitor/pika/internal/repo"
)

// monitorTargetSet 服务监控的目标探针集合
type monitorTargetSet struct {
	ids map[string]struct{}
	// all 为 true 表示未配置任何过滤条件（未指定探针且未指定标签），对所有探针生效
	all bool
}

// resolveMonitorTargetSet 解析监控任务的目标探针集合。
// 未指定探针且未指定标签时对所有探针生效；否则为指定探针与标签匹配探针的并集（可能为空，即无人执行）。
func resolveMonitorTargetSet(ctx context.Context, agentRepo *repo.AgentRepo, monitor *models.MonitorTask) (monitorTargetSet, error) {
	if len(monitor.AgentIds) == 0 && len(monitor.Tags) == 0 {
		return monitorTargetSet{all: true}, nil
	}

	set := monitorTargetSet{ids: make(map[string]struct{}, len(monitor.AgentIds))}
	for _, id := range monitor.AgentIds {
		set.ids[id] = struct{}{}
	}

	if len(monitor.Tags) > 0 {
		tagAgentIDs, err := agentRepo.FindIDsByTags(ctx, monitor.Tags)
		if err != nil {
			return set, err
		}
		for _, id := range tagAgentIDs {
			set.ids[id] = struct{}{}
		}
	}

	return set, nil
}

// Contains 判断探针是否在目标集合内（未配置过滤条件时始终为 true）
func (s monitorTargetSet) Contains(agentID string) bool {
	if s.all {
		return true
	}
	_, ok := s.ids[agentID]
	return ok
}
