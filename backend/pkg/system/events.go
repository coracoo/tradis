package system

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/client"
)

var (
	logFilePath = filepath.Join("data", "docker.logs")
	mutex       sync.Mutex
)

// LogEntry 定义日志条目结构
type LogEntry struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	TypeClass string `json:"typeClass"`
	Time      string `json:"time"`
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp"`
}

// StartEventLogger 启动事件监听和日志记录
func StartEventLogger() {
	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(logFilePath), 0755); err != nil {
		fmt.Printf("Error creating log directory: %v\n", err)
	}

	// 初始填充历史日志
	fillHistoryLogs()

	// 启动实时监听
	go func() {
		for {
			watchEvents()
			// 如果连接断开，等待一段时间后重试
			time.Sleep(5 * time.Second)
		}
	}()
}

func fillHistoryLogs() {
	// 检查文件是否已存在且有内容
	if info, err := os.Stat(logFilePath); err == nil && info.Size() > 0 {
		return
	}

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		fmt.Printf("Error creating docker client: %v\n", err)
		return
	}
	defer cli.Close()

	// 获取最近24小时的事件
	msgs, errs := cli.Events(context.Background(), types.EventsOptions{
		Since: time.Now().Add(-24 * time.Hour).Format(time.RFC3339),
	})

	// 收集所有历史事件
	var events []events.Message
loop:
	for {
		select {
		case event := <-msgs:
			events = append(events, event)
		case err := <-errs:
			if err != nil && err != io.EOF {
				fmt.Printf("Error reading history events: %v\n", err)
			}
			break loop
		case <-time.After(2 * time.Second):
			break loop
		}
	}

	// 写入文件
	for _, event := range events {
		processEvent(event)
	}
}

func watchEvents() {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		fmt.Printf("Error creating docker client: %v\n", err)
		return
	}
	defer cli.Close()

	// 从现在开始监听
	msgs, errs := cli.Events(context.Background(), types.EventsOptions{
		Since: time.Now().Format(time.RFC3339),
	})

	for {
		select {
		case event := <-msgs:
			processEvent(event)
		case err := <-errs:
			if err != nil {
				fmt.Printf("Error reading docker events: %v\n", err)
				return
			}
		}
	}
}

func processEvent(event events.Message) {
	// 只记录关心的事件类型
	if event.Type != "container" {
		return
	}

	// 过滤逻辑
	eventType := "info"
	typeClass := "info"

	switch event.Action {
	case "start", "create", "unpause":
		eventType = "success"
		typeClass = "success"
	case "stop", "die", "kill", "pause", "oom":
		eventType = "warning"
		typeClass = "warning"
	case "destroy", "delete":
		eventType = "error"
		typeClass = "danger"
	default:
		// 忽略其他事件以减少噪音
		return
	}

	timeStr := time.Unix(event.Time, 0).Format("15:04:05")
	name := event.Actor.Attributes["name"]
	if name == "" && len(event.ID) >= 12 {
		name = event.ID[:12]
	}

	entry := LogEntry{
		ID:        fmt.Sprintf("%s-%d", event.ID, event.TimeNano),
		Type:      eventType,
		TypeClass: typeClass,
		Time:      timeStr,
		Message:   fmt.Sprintf("%s %s: %s", event.Type, event.Action, name),
		Timestamp: event.Time,
	}

	appendLog(entry)
}

func appendLog(entry LogEntry) {
	mutex.Lock()
	defer mutex.Unlock()

	// 检查文件大小，如果超过 5MB 则轮转
	if info, err := os.Stat(logFilePath); err == nil && info.Size() > 5*1024*1024 {
		os.Rename(logFilePath, logFilePath+".1")
	}

	file, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("Error opening log file: %v\n", err)
		return
	}
	defer file.Close()

	data, err := json.Marshal(entry)
	if err != nil {
		return
	}

	file.Write(data)
	file.WriteString("\n")
}

// GetRecentLogs 从文件读取最近的日志
func GetRecentLogs(limit int) ([]LogEntry, error) {
	mutex.Lock()
	defer mutex.Unlock()

	var allLogs []LogEntry

	// 读取主日志文件
	readLogsFromFile(logFilePath, &allLogs)
	
	// 如果不够，且有备份文件，也读取备份文件
	if len(allLogs) < limit {
		readLogsFromFile(logFilePath+".1", &allLogs)
	}

	// 按时间戳排序（因为读取多个文件可能顺序不对，或者文件中可能有乱序）
	sort.Slice(allLogs, func(i, j int) bool {
		return allLogs[i].Timestamp > allLogs[j].Timestamp
	})

	if len(allLogs) > limit {
		allLogs = allLogs[:limit]
	}

	return allLogs, nil
}

func readLogsFromFile(path string, logs *[]LogEntry) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var entry LogEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err == nil {
			*logs = append(*logs, entry)
		}
	}
}
