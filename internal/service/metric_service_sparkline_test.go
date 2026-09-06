package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pika-monitor/pika/internal/vmclient"
)

func TestAggregateMonitorSparklines(t *testing.T) {
	result := &vmclient.QueryResult{
		Data: vmclient.ResultData{Result: []vmclient.Result{
			{
				Metric: map[string]string{"monitor_id": "monitor-a", "agent_id": "agent-1"},
				Values: [][]interface{}{{float64(100), "100"}, {float64(160), "200"}},
			},
			{
				Metric: map[string]string{"monitor_id": "monitor-a", "agent_id": "agent-2"},
				Values: [][]interface{}{{float64(100), "300"}, {float64(160), "400"}},
			},
			{
				Metric: map[string]string{"monitor_id": "private-monitor", "agent_id": "agent-1"},
				Values: [][]interface{}{{float64(100), "999"}},
			},
		}},
	}

	sparklines := aggregateMonitorSparklines(result, map[string]struct{}{"monitor-a": {}})
	points := sparklines["monitor-a"]
	if len(points) != 2 {
		t.Fatalf("expected 2 points, got %d", len(points))
	}
	if points[0].Timestamp != 100000 || points[0].Avg != 200 || points[0].Max != 300 {
		t.Fatalf("unexpected first point: %+v", points[0])
	}
	if points[1].Timestamp != 160000 || points[1].Avg != 300 || points[1].Max != 400 {
		t.Fatalf("unexpected second point: %+v", points[1])
	}
	if _, exists := sparklines["private-monitor"]; exists {
		t.Fatal("unexpected private monitor sparkline")
	}
}

func TestAggregateMonitorSparklinesSkipsMalformedValues(t *testing.T) {
	result := &vmclient.QueryResult{
		Data: vmclient.ResultData{Result: []vmclient.Result{
			{
				Metric: map[string]string{"monitor_id": "monitor-a"},
				Values: [][]interface{}{
					{float64(100), "not-a-number"},
					{"bad-timestamp", "100"},
					{float64(120), "NaN"},
					{float64(160), "50"},
				},
			},
		}},
	}

	points := aggregateMonitorSparklines(result, map[string]struct{}{"monitor-a": {}})["monitor-a"]
	if len(points) != 1 {
		t.Fatalf("expected one valid point, got %d", len(points))
	}
	if points[0].Avg != 50 || points[0].Max != 50 {
		t.Fatalf("unexpected valid point: %+v", points[0])
	}
}

func TestGetCachedMonitorSparklinesCoalescesAndCachesNormalizedIDs(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		query := r.URL.Query().Get("query")
		if !strings.Contains(query, "monitor-a") || !strings.Contains(query, "monitor-b") {
			t.Errorf("query does not contain both monitor IDs: %s", query)
		}
		time.Sleep(40 * time.Millisecond)
		if err := json.NewEncoder(w).Encode(vmclient.QueryResult{
			Status: "success",
			Data: vmclient.ResultData{ResultType: "matrix", Result: []vmclient.Result{
				{
					Metric: map[string]string{"monitor_id": "monitor-a", "agent_id": "agent-1"},
					Values: [][]interface{}{{float64(100), "125"}},
				},
			}},
		}); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	defer server.Close()

	service := &MetricService{vmClient: vmclient.NewVMClient(server.URL, time.Second, time.Second)}
	monitorIDSets := [][]string{
		{"monitor-b", "monitor-a", "monitor-a", ""},
		{"monitor-a", "monitor-b"},
	}

	start := make(chan struct{})
	var wg sync.WaitGroup
	errs := make(chan error, len(monitorIDSets))
	for _, monitorIDs := range monitorIDSets {
		wg.Add(1)
		go func(ids []string) {
			defer wg.Done()
			<-start
			_, items, err := service.GetCachedMonitorSparklines(context.Background(), ids, time.Hour, 20*time.Second)
			if err == nil && len(items["monitor-a"]) != 1 {
				t.Errorf("expected one point for monitor-a, got %+v", items)
			}
			errs <- err
		}(monitorIDs)
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("GetCachedMonitorSparklines returned error: %v", err)
		}
	}

	if got := calls.Load(); got != 1 {
		t.Fatalf("expected one coalesced VM query, got %d", got)
	}

	_, cached, err := service.GetCachedMonitorSparklines(context.Background(), []string{"monitor-b", "monitor-a"}, time.Hour, 20*time.Second)
	if err != nil {
		t.Fatalf("cached query returned error: %v", err)
	}
	cached["monitor-a"][0].Avg = 999
	_, cloned, err := service.GetCachedMonitorSparklines(context.Background(), []string{"monitor-a", "monitor-b"}, time.Hour, 20*time.Second)
	if err != nil {
		t.Fatalf("second cached query returned error: %v", err)
	}
	if cloned["monitor-a"][0].Avg != 125 {
		t.Fatalf("cached data was mutated through caller response: %+v", cloned["monitor-a"][0])
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("expected cache hits to avoid extra VM queries, got %d", got)
	}
}

func TestGetCachedMonitorSparklinesDoesNotCacheFailures(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			http.Error(w, "temporary failure", http.StatusServiceUnavailable)
			return
		}
		_ = json.NewEncoder(w).Encode(vmclient.QueryResult{Status: "success"})
	}))
	defer server.Close()

	service := &MetricService{vmClient: vmclient.NewVMClient(server.URL, time.Second, time.Second)}
	if _, _, err := service.GetCachedMonitorSparklines(context.Background(), []string{"monitor-a"}, time.Hour, 20*time.Second); err == nil {
		t.Fatal("expected the first VM query to fail")
	}
	if _, _, err := service.GetCachedMonitorSparklines(context.Background(), []string{"monitor-a"}, time.Hour, 20*time.Second); err != nil {
		t.Fatalf("expected retry after failure to succeed: %v", err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("expected failed query not to be cached, got %d calls", got)
	}
}
