package api

import (
	"context"
	"dockerpanel/backend/pkg/docker"
	"io"
	"net/http"

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
	cli, err := docker.NewDockerClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cli.Close()

	id := c.Param("id")

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
