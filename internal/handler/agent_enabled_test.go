package handler

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/pika-monitor/pika/internal/models"
	"github.com/pika-monitor/pika/internal/protocol"
	"github.com/pika-monitor/pika/internal/repo"
	"github.com/pika-monitor/pika/internal/service"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func TestHandleWebSocketMessageIgnoresDisabledAgent(t *testing.T) {
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := database.AutoMigrate(&models.Agent{}); err != nil {
		t.Fatalf("migrate agent: %v", err)
	}

	h := &AgentHandler{
		logger: zap.NewNop(),
		agentService: &service.AgentService{
			AgentRepo: repo.NewAgentRepo(database),
		},
	}

	agent := models.Agent{ID: "agent-disabled", Enabled: true}
	if err := database.Create(&agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}
	// 通过服务层切换启用状态，保证 IsAgentEnabled 的缓存被正确失效
	if err := h.agentService.UpdateAgentEnabled(context.Background(), agent.ID, false); err != nil {
		t.Fatalf("disable agent: %v", err)
	}

	invalidMetrics := json.RawMessage(`{`)
	if err := h.handleWebSocketMessage(context.Background(), agent.ID, string(protocol.MessageTypeMetrics), invalidMetrics); err != nil {
		t.Fatalf("disabled agent data should be ignored, got error: %v", err)
	}

	if err := h.agentService.UpdateAgentEnabled(context.Background(), agent.ID, true); err != nil {
		t.Fatalf("enable agent: %v", err)
	}
	if err := h.handleWebSocketMessage(context.Background(), agent.ID, string(protocol.MessageTypeMetrics), invalidMetrics); err == nil {
		t.Fatal("enabled agent data should reach the metrics parser")
	}
}

func TestDisabledAgentStatusCannotBeRefreshed(t *testing.T) {
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := database.AutoMigrate(&models.Agent{}); err != nil {
		t.Fatalf("migrate agent: %v", err)
	}

	agent := models.Agent{ID: "agent-status", Enabled: true, Status: 1}
	if err := database.Create(&agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}
	agentService := &service.AgentService{AgentRepo: repo.NewAgentRepo(database)}
	if err := agentService.UpdateAgentEnabled(context.Background(), agent.ID, false); err != nil {
		t.Fatalf("disable agent: %v", err)
	}
	if err := agentService.UpdateAgentStatus(context.Background(), agent.ID, 1); err != nil {
		t.Fatalf("refresh disabled agent status: %v", err)
	}

	var saved models.Agent
	if err := database.First(&saved, "id = ?", agent.ID).Error; err != nil {
		t.Fatalf("load agent: %v", err)
	}
	if saved.Enabled || saved.Status != 0 {
		t.Fatalf("disabled agent status changed unexpectedly: enabled=%v status=%d", saved.Enabled, saved.Status)
	}
}

func TestPublicAgentQueriesExcludeDisabledAgents(t *testing.T) {
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := database.AutoMigrate(&models.Agent{}); err != nil {
		t.Fatalf("migrate agent: %v", err)
	}

	agents := []models.Agent{
		{ID: "public-enabled", Enabled: true, Visibility: "public"},
		{ID: "public-disabled", Enabled: true, Visibility: "public"},
	}
	if err := database.Create(&agents).Error; err != nil {
		t.Fatalf("create agents: %v", err)
	}
	if err := database.Model(&models.Agent{}).Where("id = ?", "public-disabled").Update("enabled", false).Error; err != nil {
		t.Fatalf("disable agent: %v", err)
	}

	publicAgents, err := repo.NewAgentRepo(database).FindPublicAgents(context.Background())
	if err != nil {
		t.Fatalf("query public agents: %v", err)
	}
	if len(publicAgents) != 1 || publicAgents[0].ID != "public-enabled" {
		t.Fatalf("unexpected public agents: %+v", publicAgents)
	}

	authenticatedAgents, err := (&service.AgentService{AgentRepo: repo.NewAgentRepo(database)}).
		ListByAuth(context.Background(), true)
	if err != nil {
		t.Fatalf("query authenticated agents: %v", err)
	}
	if len(authenticatedAgents) != 1 || authenticatedAgents[0].ID != "public-enabled" {
		t.Fatalf("authenticated query returned disabled agents: %+v", authenticatedAgents)
	}
}
