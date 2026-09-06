package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pika-monitor/pika/internal/config"
	"go.uber.org/zap"
)

func TestOIDCProviderDiscoveryRetriesAfterFailure(t *testing.T) {
	var attempts atomic.Int32
	var provider *httptest.Server
	provider = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/openid-configuration" {
			http.NotFound(w, r)
			return
		}
		if attempts.Add(1) == 1 {
			http.Error(w, "provider is starting", http.StatusServiceUnavailable)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"issuer":                 provider.URL,
			"authorization_endpoint": provider.URL + "/authorize",
			"token_endpoint":         provider.URL + "/token",
			"jwks_uri":               provider.URL + "/keys",
		}); err != nil {
			t.Errorf("encode discovery response: %v", err)
		}
	}))
	t.Cleanup(provider.Close)

	service := NewOIDCService(zap.NewNop(), &config.AppConfig{
		OIDC: &config.OIDCConfig{
			Enabled:      true,
			Issuer:       provider.URL,
			ClientID:     "client-id",
			ClientSecret: "client-secret",
			RedirectURL:  "https://pika.example.com/admin/oidc/callback",
		},
	})

	if !service.IsEnabled() {
		t.Fatal("OIDC should be shown as enabled before Provider Discovery")
	}
	if got := attempts.Load(); got != 0 {
		t.Fatalf("NewOIDCService performed %d discovery requests, want 0", got)
	}

	if _, _, err := service.GenerateAuthURL(context.Background()); err == nil {
		t.Fatal("first GenerateAuthURL() succeeded while provider returned an error")
	}
	if !service.IsEnabled() {
		t.Fatal("transient Provider Discovery failure permanently disabled OIDC")
	}

	// 退避期内直接复用最近一次错误，不应再次请求 Provider。
	if _, _, err := service.GenerateAuthURL(context.Background()); err == nil {
		t.Fatal("GenerateAuthURL() succeeded during discovery failure backoff")
	}
	if got, want := attempts.Load(), int32(1); got != want {
		t.Fatalf("discovery attempts during backoff = %d, want %d", got, want)
	}

	// 无需真实等待退避时间；过期后下一次请求应重新执行 Discovery。
	service.providerMu.Lock()
	service.retryAfter = time.Now().Add(-time.Second)
	service.providerMu.Unlock()

	authURL, state, err := service.GenerateAuthURL(context.Background())
	if err != nil {
		t.Fatalf("GenerateAuthURL() did not recover on retry: %v", err)
	}
	parsedURL, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("parse generated auth URL: %v", err)
	}
	if got, want := parsedURL.Path, "/authorize"; got != want {
		t.Errorf("auth URL path = %q, want %q", got, want)
	}
	if got := parsedURL.Query().Get("state"); got != state {
		t.Errorf("auth URL state = %q, want %q", got, state)
	}
	if got, want := attempts.Load(), int32(2); got != want {
		t.Fatalf("discovery attempts = %d, want %d", got, want)
	}

	if _, _, err := service.GenerateAuthURL(context.Background()); err != nil {
		t.Fatalf("GenerateAuthURL() with cached Provider failed: %v", err)
	}
	if got, want := attempts.Load(), int32(2); got != want {
		t.Errorf("cached Provider caused another discovery request: got %d attempts, want %d", got, want)
	}
}

func TestOIDCProviderDiscoverySharesFailureAndAllowsCanceledWaiters(t *testing.T) {
	var attempts atomic.Int32
	discoveryStarted := make(chan struct{})
	releaseDiscovery := make(chan struct{})
	var releaseOnce sync.Once

	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		select {
		case <-discoveryStarted:
		default:
			close(discoveryStarted)
		}
		<-releaseDiscovery
		http.Error(w, "provider unavailable", http.StatusServiceUnavailable)
	}))
	t.Cleanup(provider.Close)
	// 后注册以保证清理时先解除 Handler 阻塞，再关闭测试服务器。
	t.Cleanup(func() { releaseOnce.Do(func() { close(releaseDiscovery) }) })

	service := NewOIDCService(zap.NewNop(), &config.AppConfig{
		OIDC: &config.OIDCConfig{
			Enabled:      true,
			Issuer:       provider.URL,
			ClientID:     "client-id",
			ClientSecret: "client-secret",
			RedirectURL:  "https://pika.example.com/admin/oidc/callback",
		},
	})

	const callers = 20
	results := make(chan error, callers)
	for range callers {
		go func() {
			_, _, err := service.GenerateAuthURL(context.Background())
			results <- err
		}()
	}

	select {
	case <-discoveryStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("Provider Discovery did not start")
	}

	// 等待中的调用方必须能随请求上下文取消，不能被互斥锁留住。
	canceledResult := make(chan error, 1)
	waiterCtx, cancelWaiter := context.WithCancel(context.Background())
	go func() {
		_, _, err := service.GenerateAuthURL(waiterCtx)
		canceledResult <- err
	}()
	cancelWaiter()
	select {
	case err := <-canceledResult:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled waiter error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("canceled waiter remained blocked by Provider Discovery")
	}

	releaseOnce.Do(func() { close(releaseDiscovery) })
	for range callers {
		select {
		case err := <-results:
			if err == nil {
				t.Error("GenerateAuthURL() succeeded while Provider Discovery failed")
			}
		case <-time.After(2 * time.Second):
			t.Fatal("concurrent caller did not receive shared discovery result")
		}
	}

	if got, want := attempts.Load(), int32(1); got != want {
		t.Fatalf("concurrent discovery attempts = %d, want %d", got, want)
	}
}

func TestOIDCServiceRejectsIncompleteConfig(t *testing.T) {
	service := NewOIDCService(zap.NewNop(), &config.AppConfig{
		OIDC: &config.OIDCConfig{
			Enabled: true,
			Issuer:  "https://accounts.example.com",
		},
	})

	if service.IsEnabled() {
		t.Fatal("OIDC with incomplete static config should not be enabled")
	}
}
