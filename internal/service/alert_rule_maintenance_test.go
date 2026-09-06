package service

import (
	"strings"
	"testing"
	"time"
)

func TestEffectiveAlertConfigIsInMaintenance(t *testing.T) {
	tests := []struct {
		name   string
		config EffectiveAlertConfig
		hour   int
		minute int
		wantIn bool
	}{
		{
			name: "未启用",
			config: EffectiveAlertConfig{
				MaintenanceStartTime: "02:00",
				MaintenanceEndTime:   "02:20",
			},
			hour: 2, minute: 10, wantIn: false,
		},
		{
			name: "普通时段开始边界",
			config: EffectiveAlertConfig{
				MaintenanceEnabled:   true,
				MaintenanceStartTime: "02:00",
				MaintenanceEndTime:   "02:20",
			},
			hour: 2, minute: 0, wantIn: true,
		},
		{
			name: "普通时段结束边界",
			config: EffectiveAlertConfig{
				MaintenanceEnabled:   true,
				MaintenanceStartTime: "02:00",
				MaintenanceEndTime:   "02:20",
			},
			hour: 2, minute: 20, wantIn: false,
		},
		{
			name: "跨天时段开始日期部分",
			config: EffectiveAlertConfig{
				MaintenanceEnabled:   true,
				MaintenanceStartTime: "23:50",
				MaintenanceEndTime:   "00:20",
			},
			hour: 23, minute: 55, wantIn: true,
		},
		{
			name: "跨天时段次日部分",
			config: EffectiveAlertConfig{
				MaintenanceEnabled:   true,
				MaintenanceStartTime: "23:50",
				MaintenanceEndTime:   "00:20",
			},
			hour: 0, minute: 10, wantIn: true,
		},
		{
			name: "跨天时段结束边界",
			config: EffectiveAlertConfig{
				MaintenanceEnabled:   true,
				MaintenanceStartTime: "23:50",
				MaintenanceEndTime:   "00:20",
			},
			hour: 0, minute: 20, wantIn: false,
		},
		{
			name: "无效配置不生效",
			config: EffectiveAlertConfig{
				MaintenanceEnabled:   true,
				MaintenanceStartTime: "invalid",
				MaintenanceEndTime:   "02:20",
			},
			hour: 2, minute: 10, wantIn: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Date(2026, time.August, 23, tt.hour, tt.minute, 0, 0, time.Local)
			if got := tt.config.IsInMaintenance(now); got != tt.wantIn {
				t.Fatalf("IsInMaintenance() = %v, want %v", got, tt.wantIn)
			}
		})
	}
}

func TestConditionStartAfterMaintenance(t *testing.T) {
	t.Run("普通时段后截断到当天结束时间", func(t *testing.T) {
		config := EffectiveAlertConfig{
			MaintenanceEnabled:   true,
			MaintenanceStartTime: "02:00",
			MaintenanceEndTime:   "02:20",
		}
		now := time.Date(2026, time.August, 23, 2, 30, 0, 0, time.Local)
		conditionStart := time.Date(2026, time.August, 23, 1, 50, 0, 0, time.Local).UnixMilli()
		want := time.Date(2026, time.August, 23, 2, 20, 0, 0, time.Local).UnixMilli()

		if got := config.ConditionStartAfterMaintenance(now, conditionStart); got != want {
			t.Fatalf("ConditionStartAfterMaintenance() = %d, want %d", got, want)
		}
	})

	t.Run("维护结束后的异常起点保持不变", func(t *testing.T) {
		config := EffectiveAlertConfig{
			MaintenanceEnabled:   true,
			MaintenanceStartTime: "02:00",
			MaintenanceEndTime:   "02:20",
		}
		now := time.Date(2026, time.August, 23, 2, 30, 0, 0, time.Local)
		conditionStart := time.Date(2026, time.August, 23, 2, 25, 0, 0, time.Local).UnixMilli()

		if got := config.ConditionStartAfterMaintenance(now, conditionStart); got != conditionStart {
			t.Fatalf("ConditionStartAfterMaintenance() = %d, want %d", got, conditionStart)
		}
	})

	t.Run("跨天时段后截断到次日结束时间", func(t *testing.T) {
		config := EffectiveAlertConfig{
			MaintenanceEnabled:   true,
			MaintenanceStartTime: "23:50",
			MaintenanceEndTime:   "00:20",
		}
		now := time.Date(2026, time.August, 24, 0, 30, 0, 0, time.Local)
		conditionStart := time.Date(2026, time.August, 23, 23, 40, 0, 0, time.Local).UnixMilli()
		want := time.Date(2026, time.August, 24, 0, 20, 0, 0, time.Local).UnixMilli()

		if got := config.ConditionStartAfterMaintenance(now, conditionStart); got != want {
			t.Fatalf("ConditionStartAfterMaintenance() = %d, want %d", got, want)
		}
	})
}

func TestNormalizeRuleRequestMaintenance(t *testing.T) {
	tests := []struct {
		name      string
		startTime string
		endTime   string
		wantError string
	}{
		{name: "合法普通时段", startTime: "02:00", endTime: "02:20"},
		{name: "合法跨天时段", startTime: "23:50", endTime: "00:20"},
		{name: "开始时间为空", startTime: "", endTime: "02:20", wantError: "开始时间格式不正确"},
		{name: "结束时间无效", startTime: "02:00", endTime: "25:00", wantError: "结束时间格式不正确"},
		{name: "起止时间相同", startTime: "02:00", endTime: "02:00", wantError: "不能相同"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &AlertRuleRequest{
				Name:                 "测试规则",
				TargetType:           "all",
				MaintenanceEnabled:   true,
				MaintenanceStartTime: tt.startTime,
				MaintenanceEndTime:   tt.endTime,
			}
			err := (&AlertRuleService{}).normalizeRuleRequest(req)
			if tt.wantError == "" {
				if err != nil {
					t.Fatalf("normalizeRuleRequest() unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("normalizeRuleRequest() error = %v, want contains %q", err, tt.wantError)
			}
		})
	}
}

func TestNormalizeRuleRequestMaintenanceDisabledDefaults(t *testing.T) {
	req := &AlertRuleRequest{
		Name:       "测试规则",
		TargetType: "all",
	}
	if err := (&AlertRuleService{}).normalizeRuleRequest(req); err != nil {
		t.Fatalf("normalizeRuleRequest() unexpected error: %v", err)
	}
	if req.MaintenanceStartTime != "02:00" || req.MaintenanceEndTime != "02:20" {
		t.Fatalf(
			"maintenance defaults = (%q, %q), want (%q, %q)",
			req.MaintenanceStartTime,
			req.MaintenanceEndTime,
			"02:00",
			"02:20",
		)
	}
}
