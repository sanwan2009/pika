package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/pika-monitor/pika/internal/config"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

const (
	oidcProviderInitTimeout = 10 * time.Second
	oidcProviderRetryDelay  = 5 * time.Second
)

type oidcProviderInitAttempt struct {
	done chan struct{}
	err  error
}

// OIDCService OIDC 认证服务
type OIDCService struct {
	logger       *zap.Logger
	config       *config.OIDCConfig
	providerMu   sync.Mutex
	oauth2Config oauth2.Config
	verifier     *oidc.IDTokenVerifier
	initAttempt  *oidcProviderInitAttempt
	lastInitErr  error
	retryAfter   time.Time
	stateMu      sync.Mutex
	stateStore   map[string]time.Time // 简单的 state 存储（生产环境应使用 Redis 等）
}

// NewOIDCService 创建 OIDC 服务
func NewOIDCService(logger *zap.Logger, appConfig *config.AppConfig) *OIDCService {
	if appConfig.OIDC == nil || !appConfig.OIDC.Enabled {
		logger.Info("OIDC 认证未启用")
		return &OIDCService{
			logger:     logger,
			stateStore: make(map[string]time.Time),
		}
	}

	if err := appConfig.OIDC.Validate(); err != nil {
		logger.Error("OIDC 配置不完整，OIDC 认证将被禁用", zap.Error(err))
		return &OIDCService{
			logger:     logger,
			stateStore: make(map[string]time.Time),
		}
	}

	// 配置在应用生命周期内只读，复制一份以避免调用方意外修改。
	oidcConfig := *appConfig.OIDC
	logger.Info("OIDC 配置已加载，Provider 将在首次登录时初始化", zap.String("issuer", oidcConfig.Issuer))

	return &OIDCService{
		logger:     logger,
		config:     &oidcConfig,
		stateStore: make(map[string]time.Time),
	}
}

// IsEnabled 检查 OIDC 是否启用
func (s *OIDCService) IsEnabled() bool {
	return s.config != nil && s.config.Enabled
}

// GenerateAuthURL 生成认证 URL
func (s *OIDCService) GenerateAuthURL(ctx context.Context) (string, string, error) {
	if !s.IsEnabled() {
		return "", "", errors.New("OIDC 未启用")
	}

	oauth2Config, _, err := s.runtimeConfig(ctx)
	if err != nil {
		return "", "", err
	}

	// 生成随机 state
	state, err := s.generateState()
	if err != nil {
		return "", "", fmt.Errorf("生成 state 失败: %w", err)
	}

	// 存储 state（有效期 10 分钟）
	s.stateMu.Lock()
	s.stateStore[state] = time.Now().Add(10 * time.Minute)
	s.cleanExpiredStatesLocked()
	s.stateMu.Unlock()

	authURL := oauth2Config.AuthCodeURL(state)
	return authURL, state, nil
}

// ExchangeCode 交换授权码获取 token 和用户信息
func (s *OIDCService) ExchangeCode(ctx context.Context, code, state string) (string, string, error) {
	if !s.IsEnabled() {
		return "", "", errors.New("OIDC 未启用")
	}

	oauth2Config, verifier, err := s.runtimeConfig(ctx)
	if err != nil {
		return "", "", err
	}

	// state 仅允许消费一次，校验和删除必须是原子操作。
	if !s.consumeState(state) {
		return "", "", errors.New("无效的 state")
	}

	// 交换授权码
	oauth2Token, err := oauth2Config.Exchange(ctx, code)
	if err != nil {
		return "", "", fmt.Errorf("交换授权码失败: %w", err)
	}

	// 提取 ID Token
	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		return "", "", errors.New("未获取到 ID Token")
	}

	// 验证 ID Token
	idToken, err := verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return "", "", fmt.Errorf("验证 ID Token 失败: %w", err)
	}

	// 提取用户信息
	var claims struct {
		Email             string `json:"email"`
		EmailVerified     bool   `json:"email_verified"`
		Name              string `json:"name"`
		PreferredUsername string `json:"preferred_username"`
	}

	if err := idToken.Claims(&claims); err != nil {
		return "", "", fmt.Errorf("解析 claims 失败: %w", err)
	}

	// 确定用户标识（优先使用 email，其次 preferred_username，最后使用 subject）
	username := claims.Email
	if username == "" {
		username = claims.PreferredUsername
	}
	if username == "" {
		username = idToken.Subject
	}

	nickname := claims.Name
	if nickname == "" {
		nickname = username
	}

	s.logger.Info("OIDC 认证成功",
		zap.String("username", username),
		zap.String("nickname", nickname),
		zap.String("subject", idToken.Subject))

	return username, nickname, nil
}

// runtimeConfig 返回可用于当前请求的 OIDC 运行时配置。
// 并发请求共享同一次 Discovery，等待方可以随请求上下文取消；失败后经过短暂退避再重试。
func (s *OIDCService) runtimeConfig(ctx context.Context) (oauth2.Config, *oidc.IDTokenVerifier, error) {
	for {
		if err := ctx.Err(); err != nil {
			return oauth2.Config{}, nil, err
		}

		s.providerMu.Lock()
		if s.verifier != nil {
			oauth2Config, verifier := s.oauth2Config, s.verifier
			s.providerMu.Unlock()
			return oauth2Config, verifier, nil
		}

		if s.initAttempt == nil && s.lastInitErr != nil && time.Now().Before(s.retryAfter) {
			err := s.lastInitErr
			s.providerMu.Unlock()
			return oauth2.Config{}, nil, err
		}

		attempt := s.initAttempt
		if attempt == nil {
			attempt = &oidcProviderInitAttempt{done: make(chan struct{})}
			s.initAttempt = attempt
			go s.initializeProvider(attempt)
		}
		s.providerMu.Unlock()

		select {
		case <-attempt.done:
			if attempt.err != nil {
				// 即使等待方被延迟调度到退避期之后，也共享它实际等待的这次失败结果。
				return oauth2.Config{}, nil, attempt.err
			}
			// 初始化结果已发布，重新读取成功的运行时配置。
		case <-ctx.Done():
			return oauth2.Config{}, nil, ctx.Err()
		}
	}
}

// initializeProvider 在独立、受限时长的上下文中执行 Discovery，避免首个调用方取消时影响其他等待者。
func (s *OIDCService) initializeProvider(attempt *oidcProviderInitAttempt) {
	discoveryCtx, cancel := context.WithTimeout(context.Background(), oidcProviderInitTimeout)
	defer cancel()

	provider, err := oidc.NewProvider(discoveryCtx, s.config.Issuer)
	if err != nil {
		err = fmt.Errorf("初始化 OIDC Provider 失败: %w", err)
	}

	var oauth2Config oauth2.Config
	var verifier *oidc.IDTokenVerifier
	if err == nil {
		oauth2Config = oauth2.Config{
			ClientID:     s.config.ClientID,
			ClientSecret: s.config.ClientSecret,
			RedirectURL:  s.config.RedirectURL,
			Endpoint:     provider.Endpoint(),
			Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
		}
		verifier = provider.Verifier(&oidc.Config{ClientID: s.config.ClientID})
	}

	s.providerMu.Lock()
	if err != nil {
		s.lastInitErr = err
		s.retryAfter = time.Now().Add(oidcProviderRetryDelay)
	} else {
		s.oauth2Config = oauth2Config
		s.verifier = verifier
		s.lastInitErr = nil
		s.retryAfter = time.Time{}
	}
	attempt.err = err
	s.initAttempt = nil
	close(attempt.done)
	s.providerMu.Unlock()

	if err != nil {
		s.logger.Warn("初始化 OIDC Provider 失败，退避后允许重试",
			zap.Duration("retryAfter", oidcProviderRetryDelay),
			zap.Error(err))
		return
	}
	s.logger.Info("OIDC Provider 初始化成功", zap.String("issuer", s.config.Issuer))
}

// generateState 生成随机 state
func (s *OIDCService) generateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// consumeState 验证并消费 state。
func (s *OIDCService) consumeState(state string) bool {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()

	expiresAt, exists := s.stateStore[state]
	if !exists {
		return false
	}
	delete(s.stateStore, state)
	return time.Now().Before(expiresAt)
}

// cleanExpiredStatesLocked 清理过期的 state，调用方必须持有 stateMu。
func (s *OIDCService) cleanExpiredStatesLocked() {
	now := time.Now()
	for state, expiresAt := range s.stateStore {
		if now.After(expiresAt) {
			delete(s.stateStore, state)
		}
	}
}
