package service

import (
	"testing"
	"time"

	"github.com/pika-monitor/pika/internal/models"
)

func TestAgentOfflineConditionUsesConnectionStatus(t *testing.T) {
	now := time.Date(2026, time.August, 31, 22, 31, 1, 0, time.Local).UnixMilli()
	staleLastSeen := now - int64(306*time.Second/time.Millisecond)
	config := &EffectiveAlertConfig{}

	t.Run("在线探针不因去抖后的旧心跳时间误判离线", func(t *testing.T) {
		agent := &models.Agent{Status: 1, LastSeenAt: staleLastSeen}

		isOffline, startTime, seconds := agentOfflineCondition(config, agent, now)
		if isOffline || startTime != 0 || seconds != 0 {
			t.Fatalf(
				"agentOfflineCondition() = (%v, %d, %d), want (false, 0, 0)",
				isOffline,
				startTime,
				seconds,
			)
		}
	})

	t.Run("离线探针从最后状态更新时间累计", func(t *testing.T) {
		agent := &models.Agent{Status: 0, LastSeenAt: staleLastSeen}

		isOffline, startTime, seconds := agentOfflineCondition(config, agent, now)
		if !isOffline || startTime != staleLastSeen || seconds != 306 {
			t.Fatalf(
				"agentOfflineCondition() = (%v, %d, %d), want (true, %d, 306)",
				isOffline,
				startTime,
				seconds,
				staleLastSeen,
			)
		}
	})
}
