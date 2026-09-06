package config

import (
	"strings"
	"testing"
)

func TestOIDCConfigValidate(t *testing.T) {
	tests := []struct {
		name        string
		config      *OIDCConfig
		wantMissing []string
	}{
		{
			name:   "nil config",
			config: nil,
		},
		{
			name:   "disabled config",
			config: &OIDCConfig{},
		},
		{
			name: "complete config",
			config: &OIDCConfig{
				Enabled:      true,
				Issuer:       "https://accounts.example.com",
				ClientID:     "client-id",
				ClientSecret: "client-secret",
				RedirectURL:  "https://pika.example.com/admin/oidc/callback",
			},
		},
		{
			name: "reports every missing field",
			config: &OIDCConfig{
				Enabled: true,
				Issuer:  "  ",
			},
			wantMissing: []string{"Issuer", "ClientID", "ClientSecret", "RedirectURL"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.config.Validate()
			if len(test.wantMissing) == 0 {
				if err != nil {
					t.Fatalf("Validate() returned unexpected error: %v", err)
				}
				return
			}

			if err == nil {
				t.Fatal("Validate() returned nil, want missing field error")
			}
			for _, field := range test.wantMissing {
				if !strings.Contains(err.Error(), field) {
					t.Errorf("Validate() error %q does not mention %s", err, field)
				}
			}
		})
	}
}
