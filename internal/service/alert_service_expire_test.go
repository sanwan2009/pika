package service

import (
	"testing"
	"time"
)

func TestAgentExpireReminderDay(t *testing.T) {
	now := time.Date(2026, time.August, 23, 12, 0, 0, 0, time.Local).UnixMilli()
	dayMs := int64(24 * time.Hour / time.Millisecond)

	tests := []struct {
		name       string
		expireTime int64
		wantDay    int
		wantOK     bool
	}{
		{name: "未配置到期时间", expireTime: 0, wantDay: 0, wantOK: false},
		{name: "超过七天", expireTime: now + 8*dayMs, wantDay: 0, wantOK: false},
		{name: "正好七天", expireTime: now + 7*dayMs, wantDay: 7, wantOK: true},
		{name: "七天窗口内", expireTime: now + 5*dayMs, wantDay: 7, wantOK: true},
		{name: "正好三天", expireTime: now + 3*dayMs, wantDay: 3, wantOK: true},
		{name: "三天窗口内", expireTime: now + 2*dayMs, wantDay: 3, wantOK: true},
		{name: "正好一天", expireTime: now + dayMs, wantDay: 1, wantOK: true},
		{name: "不足一天", expireTime: now + dayMs/2, wantDay: 1, wantOK: true},
		{name: "已经到期", expireTime: now - dayMs, wantDay: 1, wantOK: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotDay, gotOK := agentExpireReminderDay(tt.expireTime, now)
			if gotDay != tt.wantDay || gotOK != tt.wantOK {
				t.Fatalf("agentExpireReminderDay() = (%d, %v), want (%d, %v)", gotDay, gotOK, tt.wantDay, tt.wantOK)
			}
		})
	}
}
