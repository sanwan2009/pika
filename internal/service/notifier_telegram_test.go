package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"go.uber.org/zap"
)

func TestBuildTelegramWebhookURL(t *testing.T) {
	tests := []struct {
		name       string
		apiBaseURL string
		want       string
		wantErr    bool
	}{
		{name: "default", want: "https://api.telegram.org/bot123:abc/sendMessage"},
		{name: "custom host", apiBaseURL: "https://telegram.example.com/", want: "https://telegram.example.com/bot123:abc/sendMessage"},
		{name: "custom path", apiBaseURL: "http://telegram.example.com/api", want: "http://telegram.example.com/api/bot123:abc/sendMessage"},
		{name: "unsupported scheme", apiBaseURL: "ftp://telegram.example.com", wantErr: true},
		{name: "query is rejected", apiBaseURL: "https://telegram.example.com/?key=value", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildTelegramWebhookURL("123:abc", tt.apiBaseURL)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("buildTelegramWebhookURL() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("buildTelegramWebhookURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNewTelegramHTTPClient(t *testing.T) {
	for _, proxyURL := range []string{
		"http://user:pass@127.0.0.1:7890",
		"socks5://user:pass@127.0.0.1:1080",
	} {
		t.Run(proxyURL, func(t *testing.T) {
			client, err := newTelegramHTTPClient(proxyURL)
			if err != nil {
				t.Fatalf("newTelegramHTTPClient() error = %v", err)
			}
			if client == sharedHTTPClient {
				t.Fatal("proxied Telegram client unexpectedly uses the shared client")
			}
		})
	}

	if _, err := newTelegramHTTPClient("https://127.0.0.1:7890"); err == nil {
		t.Fatal("expected HTTPS proxy scheme to be rejected")
	}
}

func TestSendTelegramWithCustomAPIBaseURL(t *testing.T) {
	var gotBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/telegram/bot123:abc/sendMessage" {
			t.Errorf("request path = %q", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	notifier := NewNotifier(zap.NewNop())
	err := notifier.sendTelegram(context.Background(), "123:abc", "456", "hello", "", server.URL+"/telegram/")
	if err != nil {
		t.Fatalf("sendTelegram() error = %v", err)
	}
	if gotBody["chat_id"] != "456" || gotBody["text"] != "hello" {
		t.Fatalf("request body = %#v", gotBody)
	}
}

func TestSendTelegramWithHTTPProxy(t *testing.T) {
	proxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Scheme != "http" || r.URL.Host != "telegram.invalid" {
			t.Errorf("proxy request URL = %q", r.URL.String())
		}
		wantProxyAuthorization := "Basic " + base64.StdEncoding.EncodeToString([]byte("proxy-user:proxy-password"))
		if got := r.Header.Get("Proxy-Authorization"); got != wantProxyAuthorization {
			t.Errorf("Proxy-Authorization = %q, want %q", got, wantProxyAuthorization)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer proxyServer.Close()

	parsedProxyURL, err := url.Parse(proxyServer.URL)
	if err != nil {
		t.Fatal(err)
	}
	parsedProxyURL.User = url.UserPassword("proxy-user", "proxy-password")

	notifier := NewNotifier(zap.NewNop())
	err = notifier.sendTelegram(
		context.Background(),
		"123:abc",
		"456",
		"hello",
		parsedProxyURL.String(),
		"http://telegram.invalid",
	)
	if err != nil {
		t.Fatalf("sendTelegram() error = %v", err)
	}
}
