package api

import (
	"bufio"
	"context"
	"dockerpanel/backend/pkg/docker"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/gin-gonic/gin"
)

// 每次写入后刷新响应的 Writer，确保日志实时推送到前端
type flushWriter struct{ w gin.ResponseWriter }

func (fw flushWriter) Write(p []byte) (int, error) {
	n, err := fw.w.Write(p)
	fw.w.Flush()
	return n, err
}

// 修改日志接口，支持实时日志
func getContainerLogs(c *gin.Context) {
	id := c.Param("id")
	if forbidIfSelfContainer(c, id) {
		return
	}

	cli, err := docker.NewDockerClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cli.Close()

	// 先检查容器是否存在，并获取 TTY 配置
	inspect, err := cli.ContainerInspect(context.Background(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "容器不存在"})
		return
	}

	options := types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Timestamps: false,
		Tail:       "100",
	}

	logs, err := cli.ContainerLogs(context.Background(), id, options)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer logs.Close()

	// 使用纯文本流，避免事件流格式导致前端解析异常
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	fw := flushWriter{w: c.Writer}

	// 根据容器是否启用 TTY 来选择解析方式：
	// - TTY=true: 日志为原始字节流，直接复制
	// - TTY=false: 日志为多路复用流，需要通过 stdcopy 正确拆包
	if inspect.Config != nil && inspect.Config.Tty {
		_, _ = io.Copy(fw, logs)
	} else {
		_, _ = stdcopy.StdCopy(fw, fw, logs)
	}
}

// getContainerLogsEvents 以 SSE 方式推送容器日志（逐行）
func getContainerLogsEvents(c *gin.Context) {
	ctx := c.Request.Context()

	id := c.Param("id")
	if forbidIfSelfContainer(c, id) {
		return
	}

	cli, err := docker.NewDockerClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cli.Close()

	inspect, err := cli.ContainerInspect(ctx, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "容器不存在"})
		return
	}

	tail := strings.TrimSpace(c.DefaultQuery("tail", "200"))
	if tail == "" {
		tail = "200"
	}

	options := types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Timestamps: false,
		Tail:       tail,
	}

	logs, err := cli.ContainerLogs(ctx, id, options)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer logs.Close()

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("X-Accel-Buffering", "no")

	lastEventIDRaw := strings.TrimSpace(c.GetHeader("Last-Event-ID"))
	nextID := int64(1)
	if lastEventIDRaw != "" {
		if v, err := strconv.ParseInt(lastEventIDRaw, 10, 64); err == nil && v >= 0 {
			nextID = v + 1
		}
	}

	lines := make(chan string, 256)
	pr, pw := io.Pipe()

	go func() {
		defer func() { _ = pw.Close() }()

		if inspect.Config != nil && inspect.Config.Tty {
			_, _ = io.Copy(pw, logs)
			return
		}

		_, _ = stdcopy.StdCopy(pw, pw, logs)
	}()

	go func() {
		defer close(lines)
		defer func() { _ = pr.Close() }()

		scanner := bufio.NewScanner(pr)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			default:
			}
			line := strings.TrimRight(scanner.Text(), "\r\n")
			if strings.TrimSpace(line) == "" {
				continue
			}
			select {
			case <-ctx.Done():
				return
			case lines <- line:
				continue
			}
		}
	}()

	c.Stream(func(w io.Writer) bool {
		select {
		case <-ctx.Done():
			return false
		case msg, ok := <-lines:
			if !ok {
				return false
			}
			fmt.Fprintf(c.Writer, "id: %d\ndata: %s\n\n", nextID, msg)
			c.Writer.Flush()
			nextID++
			return true
		}
	})
}
