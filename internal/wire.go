//go:build wireinject
// +build wireinject

package internal

import (
	"time"

	"github.com/google/wire"
	"github.com/pika-monitor/pika/internal/config"
	"github.com/pika-monitor/pika/internal/handler"
	"github.com/pika-monitor/pika/internal/service"
	"github.com/pika-monitor/pika/internal/vmclient"
	"github.com/pika-monitor/pika/internal/websocket"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// InitializeApp 初始化应用
func InitializeApp(logger *zap.Logger, db *gorm.DB, cfg *config.AppConfig) (*AppComponents, error) {
	wire.Build(
		// VictoriaMetrics Client
		provideVMClient,

		service.NewAccountService,
		service.NewAgentService,
		service.NewUserService,
		service.NewOIDCService,
		service.NewGitHubOAuthService,
		service.NewApiKeyService,
		service.NewAlertService,
		service.NewAlertRuleService,
		service.NewPropertyService,
		service.NewNotificationService,
		service.NewMonitorService,
		service.NewTamperService,
		service.NewTrafficService,
		service.NewMetricService,
		service.NewGeoIPService,
		service.NewDDNSService,
		service.NewSSHLoginService,
		service.NewPublicIPService,
		service.NewThemeService,

		service.NewNotifier,
		// WebSocket Manager
		websocket.NewManager,

		// Handlers
		handler.NewAgentHandler,
		handler.NewAlertHandler,
		handler.NewAlertRuleHandler,
		handler.NewPropertyHandler,
		handler.NewMonitorHandler,
		handler.NewApiKeyHandler,
		handler.NewAccountHandler,
		handler.NewTamperHandler,
		handler.NewDNSProviderHandler,
		handler.NewDDNSHandler,
		handler.NewSSHLoginHandler,
		handler.NewThemeHandler,
		handler.NewWebHandler,

		// App Components
		wire.Struct(new(AppComponents), "*"),
	)
	return nil, nil
}

// AppComponents 应用组件
type AppComponents struct {
	AccountHandler     *handler.AccountHandler
	AgentHandler       *handler.AgentHandler
	ApiKeyHandler      *handler.ApiKeyHandler
	AlertHandler       *handler.AlertHandler
	AlertRuleHandler   *handler.AlertRuleHandler
	PropertyHandler    *handler.PropertyHandler
	MonitorHandler     *handler.MonitorHandler
	TamperHandler      *handler.TamperHandler
	DNSProviderHandler *handler.DNSProviderHandler
	DDNSHandler        *handler.DDNSHandler
	SSHLoginHandler    *handler.SSHLoginHandler
	ThemeHandler       *handler.ThemeHandler
	WebHandler         *handler.WebHandler

	AgentService     *service.AgentService
	TrafficService   *service.TrafficService
	MetricService    *service.MetricService
	AlertService     *service.AlertService
	AlertRuleService *service.AlertRuleService
	PropertyService  *service.PropertyService
	MonitorService   *service.MonitorService
	ApiKeyService    *service.ApiKeyService
	TamperService    *service.TamperService
	DDNSService      *service.DDNSService
	SSHLoginService  *service.SSHLoginService
	PublicIPService  *service.PublicIPService
	ThemeService     *service.ThemeService

	WSManager *websocket.Manager
	VMClient  *vmclient.VMClient
}

// provideVMClient 提供 VictoriaMetrics 客户端
func provideVMClient(cfg *config.AppConfig, logger *zap.Logger) *vmclient.VMClient {
	// 写入超时默认 10s：VM 抖动时快速失败，未确认消息由探针重放
	// 机制兜底，避免 30s 级别的停滞阻塞探针消息的串行处理
	const defaultWriteTimeout = 10 * time.Second

	// 检查配置
	if cfg.VictoriaMetrics == nil || !cfg.VictoriaMetrics.Enabled {
		logger.Info("VictoriaMetrics is not enabled, using default configuration")
		// 返回一个默认配置的客户端（用于本地开发）
		return vmclient.NewVMClient("http://localhost:8428", defaultWriteTimeout, 60*time.Second)
	}

	// 使用配置创建客户端
	writeTimeout := time.Duration(cfg.VictoriaMetrics.WriteTimeout) * time.Second
	if writeTimeout == 0 {
		writeTimeout = defaultWriteTimeout
	}

	queryTimeout := time.Duration(cfg.VictoriaMetrics.QueryTimeout) * time.Second
	if queryTimeout == 0 {
		queryTimeout = 60 * time.Second
	}

	logger.Info("VictoriaMetrics client initialized",
		zap.String("url", cfg.VictoriaMetrics.URL),
		zap.Duration("writeTimeout", writeTimeout),
		zap.Duration("queryTimeout", queryTimeout))

	return vmclient.NewVMClient(cfg.VictoriaMetrics.URL, writeTimeout, queryTimeout)
}
