package service

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/pika-monitor/pika/internal/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func TestResolveForAgentsExcludesDisabledAgents(t *testing.T) {
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := database.AutoMigrate(&models.Agent{}, &models.AlertRule{}); err != nil {
		t.Fatalf("migrate models: %v", err)
	}

	agents := []models.Agent{
		{ID: "enabled-agent", Enabled: true},
		{ID: "disabled-agent", Enabled: true},
	}
	if err := database.Create(&agents).Error; err != nil {
		t.Fatalf("create agents: %v", err)
	}
	if err := database.Model(&models.Agent{}).Where("id = ?", "disabled-agent").Update("enabled", false).Error; err != nil {
		t.Fatalf("disable agent: %v", err)
	}

	rule := models.AlertRule{
		ID:         "all-agents-rule",
		Name:       "all agents",
		Priority:   1,
		Enabled:    true,
		TargetType: models.AlertRuleTargetAll,
	}
	if err := database.Create(&rule).Error; err != nil {
		t.Fatalf("create alert rule: %v", err)
	}

	alertRuleService := NewAlertRuleService(zap.NewNop(), database, nil)
	configs, err := alertRuleService.ResolveForAgents(context.Background(), []string{"enabled-agent", "disabled-agent"})
	if err != nil {
		t.Fatalf("resolve alert rules: %v", err)
	}
	if configs["enabled-agent"] == nil {
		t.Fatal("enabled agent should match the alert rule")
	}
	if configs["disabled-agent"] != nil {
		t.Fatal("disabled agent must not match any alert rule")
	}
}
