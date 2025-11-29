package scheduler

import (
	"context"
	"sync"
	"time"

	"github.com/dushixiang/pika/internal/models"
	"github.com/dushixiang/pika/internal/service"
	"go.uber.org/zap"
)

// MonitorTask 调度任务
type MonitorTask struct {
	ID          string
	Monitor     models.MonitorTask
	NextRunTime time.Time
	Interval    time.Duration
	Running     bool
}

// MonitorScheduler 监控任务调度器
type MonitorScheduler struct {
	mu             sync.RWMutex
	tasks          map[string]*MonitorTask // taskID -> MonitorTask
	monitorService *service.MonitorService
	logger         *zap.Logger
	ctx            context.Context
	cancel         context.CancelFunc
	workerCount    int
	taskChan       chan *MonitorTask
	reloadInterval time.Duration
	tickInterval   time.Duration
}

// NewMonitorScheduler 创建监控任务调度器
func NewMonitorScheduler(monitorService *service.MonitorService, logger *zap.Logger, workerCount int) *MonitorScheduler {
	if workerCount <= 0 {
		workerCount = 5 // 默认 5 个 worker
	}

	return &MonitorScheduler{
		tasks:          make(map[string]*MonitorTask),
		monitorService: monitorService,
		logger:         logger,
		workerCount:    workerCount,
		taskChan:       make(chan *MonitorTask, 100),
		reloadInterval: 10 * time.Second, // 每 10 秒重新加载任务列表
		tickInterval:   1 * time.Second,  // 每秒检查一次
	}
}

// Start 启动调度器
func (s *MonitorScheduler) Start(ctx context.Context) {
	s.ctx, s.cancel = context.WithCancel(ctx)

	s.logger.Info("启动监控任务调度器",
		zap.Int("workerCount", s.workerCount))

	// 启动 worker pool
	for i := 0; i < s.workerCount; i++ {
		go s.worker(i)
	}

	// 首次加载任务
	s.reloadTasks()

	// 启动调度循环
	go s.scheduleLoop()

	// 启动重载循环
	go s.reloadLoop()
}

// Stop 停止调度器
func (s *MonitorScheduler) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	close(s.taskChan)
	s.logger.Info("监控任务调度器已停止")
}

// scheduleLoop 调度循环
func (s *MonitorScheduler) scheduleLoop() {
	ticker := time.NewTicker(s.tickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case now := <-ticker.C:
			s.checkAndSchedule(now)
		}
	}
}

// reloadLoop 重载循环
func (s *MonitorScheduler) reloadLoop() {
	ticker := time.NewTicker(s.reloadInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.reloadTasks()
		}
	}
}

// checkAndSchedule 检查并调度应该执行的任务
func (s *MonitorScheduler) checkAndSchedule(now time.Time) {
	s.mu.RLock()
	tasksToRun := make([]*MonitorTask, 0)

	for _, task := range s.tasks {
		// 如果任务未在运行且到了执行时间
		if !task.Running && !now.Before(task.NextRunTime) {
			tasksToRun = append(tasksToRun, task)
		}
	}
	s.mu.RUnlock()

	// 提交任务到 worker pool
	for _, task := range tasksToRun {
		s.mu.Lock()
		task.Running = true
		s.mu.Unlock()

		select {
		case s.taskChan <- task:
			// 任务已提交
		default:
			// worker pool 已满，重置运行状态
			s.mu.Lock()
			task.Running = false
			s.mu.Unlock()
			s.logger.Warn("worker pool 已满，跳过任务",
				zap.String("taskID", task.ID),
				zap.String("taskName", task.Monitor.Name))
		}
	}
}

// worker worker 协程，执行任务
func (s *MonitorScheduler) worker(id int) {
	s.logger.Debug("启动 worker", zap.Int("workerID", id))

	for {
		select {
		case <-s.ctx.Done():
			return
		case task, ok := <-s.taskChan:
			if !ok {
				return
			}
			s.executeTask(task)
		}
	}
}

// executeTask 执行任务
func (s *MonitorScheduler) executeTask(task *MonitorTask) {
	defer func() {
		// 更新下次执行时间和状态
		s.mu.Lock()
		task.NextRunTime = time.Now().Add(task.Interval)
		task.Running = false
		s.mu.Unlock()
	}()

	s.logger.Debug("执行监控任务",
		zap.String("taskID", task.ID),
		zap.String("taskName", task.Monitor.Name),
		zap.Duration("interval", task.Interval))

	// 发送监控任务到探针
	if err := s.monitorService.SendMonitorTaskToAgents(s.ctx, task.Monitor, task.Monitor.AgentIds); err != nil {
		s.logger.Error("发送监控任务失败",
			zap.String("taskID", task.ID),
			zap.String("taskName", task.Monitor.Name),
			zap.Error(err))
	}
}

// reloadTasks 重新加载任务列表
func (s *MonitorScheduler) reloadTasks() {
	// 获取所有启用的监控任务
	monitors, err := s.monitorService.FindByEnabled(context.Background(), true)
	if err != nil {
		s.logger.Error("加载监控任务失败", zap.Error(err))
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 标记当前存在的任务
	existingTasks := make(map[string]bool)
	for _, monitor := range monitors {
		existingTasks[monitor.ID] = true

		if task, exists := s.tasks[monitor.ID]; exists {
			// 任务已存在，检查是否需要更新
			if task.Monitor.Interval != monitor.Interval ||
				task.Monitor.Name != monitor.Name ||
				task.Monitor.Type != monitor.Type ||
				task.Monitor.Target != monitor.Target {
				// 任务参数已变化，更新任务
				s.logger.Info("更新监控任务",
					zap.String("taskID", monitor.ID),
					zap.String("taskName", monitor.Name))
				task.Monitor = monitor
				task.Interval = time.Duration(monitor.Interval) * time.Second
				if task.Interval <= 0 {
					task.Interval = 60 * time.Second
				}
			}
		} else {
			// 新任务，添加到调度器
			interval := time.Duration(monitor.Interval) * time.Second
			if interval <= 0 {
				interval = 60 * time.Second
			}

			s.logger.Info("添加监控任务",
				zap.String("taskID", monitor.ID),
				zap.String("taskName", monitor.Name),
				zap.Duration("interval", interval))

			s.tasks[monitor.ID] = &MonitorTask{
				ID:          monitor.ID,
				Monitor:     monitor,
				NextRunTime: time.Now(), // 立即执行一次
				Interval:    interval,
				Running:     false,
			}
		}
	}

	// 删除已不存在或已禁用的任务
	for taskID := range s.tasks {
		if !existingTasks[taskID] {
			s.logger.Info("删除监控任务", zap.String("taskID", taskID))
			delete(s.tasks, taskID)
		}
	}
}

// GetTaskCount 获取任务数量
func (s *MonitorScheduler) GetTaskCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.tasks)
}

// GetTaskStatus 获取任务状态
func (s *MonitorScheduler) GetTaskStatus() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tasks := make([]map[string]interface{}, 0, len(s.tasks))
	for _, task := range s.tasks {
		tasks = append(tasks, map[string]interface{}{
			"id":          task.ID,
			"name":        task.Monitor.Name,
			"interval":    task.Interval.Seconds(),
			"nextRunTime": task.NextRunTime.Format(time.RFC3339),
			"running":     task.Running,
		})
	}

	return map[string]interface{}{
		"totalTasks":  len(s.tasks),
		"workerCount": s.workerCount,
		"tasks":       tasks,
	}
}
