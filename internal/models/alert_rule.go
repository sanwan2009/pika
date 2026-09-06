package models

import (
	"gorm.io/datatypes"
)

// 告警规则的主机目标类型
const (
	AlertRuleTargetAll    = "all"    // 全部主机
	AlertRuleTargetAgents = "agents" // 指定主机
	AlertRuleTargetTags   = "tags"   // 按标签
)

// AlertRule 告警规则（针对一批主机的一套告警规则与通知渠道）
type AlertRule struct {
	ID                   string                                 `gorm:"primaryKey" json:"id"`                    // 规则 ID
	Name                 string                                 `gorm:"uniqueIndex" json:"name"`                 // 规则名称
	Priority             int                                    `gorm:"default:0;index" json:"priority"`         // 优先级，数字越小越优先
	Enabled              bool                                   `gorm:"default:true" json:"enabled"`             // 是否启用
	TargetType           string                                 `gorm:"default:all" json:"targetType"`           // 主机目标类型: all-全部, agents-指定主机, tags-按标签
	AgentIds             datatypes.JSONSlice[string]            `json:"agentIds"`                                // 适用主机 ID 列表（targetType=agents 时有效）
	AgentNames           []string                               `gorm:"-" json:"agentNames"`                     // 适用主机名称列表（查询时动态填充）
	Tags                 datatypes.JSONSlice[string]            `json:"tags"`                                    // 适用标签列表（targetType=tags 时有效）
	Rules                datatypes.JSONType[AlertRules]         `json:"rules"`                                   // 告警规则（7 类阈值规则）
	Channels             datatypes.JSONSlice[string]            `json:"channels"`                                // 通知渠道类型列表（空 = 所有启用渠道）
	MaskIP               bool                                   `json:"maskIP"`                                  // 通知中是否打码 IP 地址
	Notifications        datatypes.JSONType[AlertNotifications] `json:"notifications"`                           // 事件通知开关（流量/SSH登录/防篡改/机器到期）
	MaintenanceEnabled   bool                                   `gorm:"default:false" json:"maintenanceEnabled"` // 是否启用每日计划维护
	MaintenanceStartTime string                                 `json:"maintenanceStartTime"`                    // 每日计划维护开始时间（HH:mm）
	MaintenanceEndTime   string                                 `json:"maintenanceEndTime"`                      // 每日计划维护结束时间（HH:mm）
	CreatedAt            int64                                  `gorm:"autoCreateTime:milli" json:"createdAt"`   // 创建时间
	UpdatedAt            int64                                  `gorm:"autoUpdateTime:milli" json:"updatedAt"`   // 更新时间
}

func (AlertRule) TableName() string {
	return "alert_rules"
}
