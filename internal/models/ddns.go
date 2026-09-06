package models

import "gorm.io/datatypes"

// DDNSConfig DDNS 配置
type DDNSConfig struct {
	ID       string `gorm:"primaryKey" json:"id"` // 配置ID (UUID)
	AgentID  string `json:"agentId"`              // 探针ID
	Name     string `json:"name"`                 // 配置名称
	Enabled  bool   `json:"enabled"`              // 是否启用
	Provider string `json:"provider"`             // DNS服务商类型: aliyun, tencentcloud, cloudflare, huaweicloud

	// 域名配置（IPv4 和 IPv6 分开）
	DomainsIPv4 datatypes.JSONSlice[string] `json:"domainsIpv4"` // IPv4 域名列表
	DomainsIPv6 datatypes.JSONSlice[string] `json:"domainsIpv6"` // IPv6 域名列表

	// IP 获取配置
	EnableIPv4    bool   `json:"enableIpv4"`             // 是否启用 IPv4
	EnableIPv6    bool   `json:"enableIpv6"`             // 是否启用 IPv6
	IPv4GetMethod string `json:"ipv4GetMethod"`          // IPv4 获取方式: api, interface
	IPv6GetMethod string `json:"ipv6GetMethod"`          // IPv6 获取方式: api, interface
	IPv4GetValue  string `json:"ipv4GetValue,omitempty"` // IPv4 获取配置值（接口名/API URL）
	IPv6GetValue  string `json:"ipv6GetValue,omitempty"` // IPv6 获取配置值（接口名/API URL）

	CreatedAt int64 `json:"createdAt"`                             // 创建时间（时间戳毫秒）
	UpdatedAt int64 `json:"updatedAt" gorm:"autoUpdateTime:milli"` // 更新时间（时间戳毫秒）
}

func (DDNSConfig) TableName() string {
	return "ddns_configs"
}

// DDNSRecord DDNS 更新记录
type DDNSRecord struct {
	ID           string `gorm:"primaryKey" json:"id"`   // 记录ID
	ConfigID     string `gorm:"index" json:"configId"`  // 配置ID
	AgentID      string `gorm:"index" json:"agentId"`   // 探针ID
	Domain       string `gorm:"index" json:"domain"`    // 域名
	RecordType   string `json:"recordType"`             // 记录类型: A, AAAA
	OldIP        string `json:"oldIp,omitempty"`        // 旧IP
	NewIP        string `json:"newIp"`                  // 新IP
	Status       string `json:"status"`                 // 更新状态: success, failed
	ErrorMessage string `json:"errorMessage,omitempty"` // 错误信息
	CreatedAt    int64  `gorm:"index" json:"createdAt"` // 创建时间（时间戳毫秒）
}

func (DDNSRecord) TableName() string {
	return "ddns_records"
}
