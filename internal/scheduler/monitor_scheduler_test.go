package scheduler

import (
	"sync"
	"testing"

	"go.uber.org/zap"
)

func TestMonitorSchedulerConcurrentUpdateKeepsSingleCronEntry(t *testing.T) {
	scheduler := NewMonitorScheduler(nil, zap.NewNop())
	const monitorID = "monitor-1"

	if err := scheduler.AddTask(monitorID, 3600); err != nil {
		t.Fatalf("AddTask() error = %v", err)
	}
	scheduler.mu.RLock()
	firstGeneration := scheduler.tasks[monitorID]
	scheduler.mu.RUnlock()

	const updates = 64
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(updates)
	for i := 0; i < updates; i++ {
		interval := 3600 + i
		go func() {
			defer wg.Done()
			<-start
			if err := scheduler.UpdateTask(monitorID, interval); err != nil {
				t.Errorf("UpdateTask() error = %v", err)
			}
		}()
	}

	close(start)
	wg.Wait()

	if got := scheduler.GetTaskCount(); got != 1 {
		t.Fatalf("GetTaskCount() = %d, want 1", got)
	}

	entries := scheduler.cron.Entries()
	if len(entries) != 1 {
		t.Fatalf("len(cron.Entries()) = %d, want 1", len(entries))
	}

	scheduler.mu.RLock()
	task := scheduler.tasks[monitorID]
	scheduler.mu.RUnlock()
	if task == nil {
		t.Fatal("task is missing from scheduler map")
	}
	if task.EntryID != entries[0].ID {
		t.Fatalf("tracked entry ID = %d, cron entry ID = %d", task.EntryID, entries[0].ID)
	}
	if scheduler.isCurrentTask(firstGeneration) {
		t.Fatal("replaced task generation must not remain executable")
	}
	if !scheduler.isCurrentTask(task) {
		t.Fatal("tracked task generation must remain executable")
	}
}
