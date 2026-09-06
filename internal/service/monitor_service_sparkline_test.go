package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/pika-monitor/pika/internal/models"
	"github.com/pika-monitor/pika/internal/vmclient"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func TestGetMonitorSparklinesByAuthExcludesPrivateMonitors(t *testing.T) {
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := database.AutoMigrate(&models.MonitorTask{}); err != nil {
		t.Fatalf("migrate monitor table: %v", err)
	}
	for _, monitor := range []models.MonitorTask{
		{ID: "public-monitor", Name: "Public", Enabled: true, Visibility: "public"},
		{ID: "private-monitor", Name: "Private", Enabled: true, Visibility: "private"},
	} {
		if err := database.Create(&monitor).Error; err != nil {
			t.Fatalf("create monitor %s: %v", monitor.ID, err)
		}
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		if !strings.Contains(query, "public-monitor") {
			t.Errorf("query does not contain public monitor: %s", query)
		}
		if strings.Contains(query, "private-monitor") {
			t.Errorf("anonymous query contains private monitor: %s", query)
		}
		_ = json.NewEncoder(w).Encode(vmclient.QueryResult{
			Status: "success",
			Data: vmclient.ResultData{ResultType: "matrix", Result: []vmclient.Result{
				{
					Metric: map[string]string{"monitor_id": "public-monitor"},
					Values: [][]interface{}{{float64(100), "100"}},
				},
				{
					Metric: map[string]string{"monitor_id": "private-monitor"},
					Values: [][]interface{}{{float64(100), "999"}},
				},
			}},
		})
	}))
	defer server.Close()

	metricService := &MetricService{vmClient: vmclient.NewVMClient(server.URL, time.Second, time.Second)}
	monitorService := NewMonitorService(zap.NewNop(), database, metricService, nil)
	result, err := monitorService.GetMonitorSparklinesByAuth(t.Context(), false)
	if err != nil {
		t.Fatalf("GetMonitorSparklinesByAuth returned error: %v", err)
	}
	if len(result.Items["public-monitor"]) != 1 {
		t.Fatalf("expected public monitor sparkline, got %+v", result.Items)
	}
	if _, ok := result.Items["private-monitor"]; ok {
		t.Fatalf("private monitor leaked into anonymous response: %+v", result.Items)
	}
}

func TestListByAuthDoesNotRequireVictoriaMetrics(t *testing.T) {
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := database.AutoMigrate(&models.MonitorTask{}); err != nil {
		t.Fatalf("migrate monitor table: %v", err)
	}
	monitor := models.MonitorTask{
		ID:         "public-monitor",
		Name:       "Public",
		Enabled:    true,
		Visibility: "public",
	}
	if err := database.Create(&monitor).Error; err != nil {
		t.Fatalf("create monitor: %v", err)
	}

	metricService := NewMetricService(zap.NewNop(), database, nil, nil, nil)
	monitorService := NewMonitorService(zap.NewNop(), database, metricService, nil)
	items, err := monitorService.ListByAuth(t.Context(), false)
	if err != nil {
		t.Fatalf("ListByAuth returned error: %v", err)
	}
	if len(items) != 1 || items[0].ID != monitor.ID {
		t.Fatalf("unexpected public monitor list: %+v", items)
	}
}
