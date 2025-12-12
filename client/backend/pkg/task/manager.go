package task

import (
	"fmt"
	"sync"
	"time"
)

type TaskStatus string

const (
	StatusPending   TaskStatus = "pending"
	StatusRunning   TaskStatus = "running"
	StatusSuccess   TaskStatus = "success"
	StatusFailed    TaskStatus = "error"
	StatusCompleted TaskStatus = "completed" // 最终状态，无论成功失败
)

type LogEntry struct {
	Time    time.Time `json:"time"`
	Type    string    `json:"type"` // info, warning, error, success
	Message string    `json:"message"`
}

type Task struct {
	ID        string      `json:"id"`
	Type      string      `json:"type"`
	Status    TaskStatus  `json:"status"`
	Logs      []LogEntry  `json:"logs"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
	Result    interface{} `json:"result,omitempty"`
	Error     string      `json:"error,omitempty"`

	// logChan 用于实时推送日志给 SSE 客户端
	logChan chan LogEntry
	// closeChan 用于通知任务结束
	closeChan chan struct{}
	mu        sync.RWMutex
}

type Manager struct {
	tasks map[string]*Task
	mu    sync.RWMutex
}

var (
	GlobalManager *Manager
	once          sync.Once
)

func GetManager() *Manager {
	once.Do(func() {
		GlobalManager = &Manager{
			tasks: make(map[string]*Task),
		}
	})
	return GlobalManager
}

func (m *Manager) CreateTask(taskType string) *Task {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := fmt.Sprintf("%d", time.Now().UnixNano())
	task := &Task{
		ID:        id,
		Type:      taskType,
		Status:    StatusPending,
		Logs:      make([]LogEntry, 0),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		logChan:   make(chan LogEntry, 100), // 缓冲通道
		closeChan: make(chan struct{}),
	}
	m.tasks[id] = task
	return task
}

func (m *Manager) GetTask(id string) *Task {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.tasks[id]
}

// AddLog 添加日志并推送到通道
func (t *Task) AddLog(logType, message string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	entry := LogEntry{
		Time:    time.Now(),
		Type:    logType,
		Message: message,
	}
	t.Logs = append(t.Logs, entry)
	t.UpdatedAt = time.Now()

	// 非阻塞发送日志，避免阻塞任务执行
	select {
	case t.logChan <- entry:
	default:
		// 通道已满，丢弃日志或处理
	}
}

// UpdateStatus 更新任务状态
func (t *Task) UpdateStatus(status TaskStatus) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Status = status
	t.UpdatedAt = time.Now()
}

// Finish 标记任务完成（成功或失败）并关闭通道
func (t *Task) Finish(status TaskStatus, result interface{}, errStr string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.Status = status
	t.Result = result
	t.Error = errStr
	t.UpdatedAt = time.Now()

	close(t.closeChan)
	close(t.logChan)
}

// Subscribe 订阅任务日志流
func (t *Task) Subscribe() (<-chan LogEntry, <-chan struct{}) {
	return t.logChan, t.closeChan
}

// GetLogs 获取当前所有日志（线程安全）
func (t *Task) GetLogs() []LogEntry {
	t.mu.RLock()
	defer t.mu.RUnlock()
	// 返回副本以避免并发问题
	logs := make([]LogEntry, len(t.Logs))
	copy(logs, t.Logs)
	return logs
}
