package api

import (
    "context"
    "dockerpanel/backend/pkg/docker"
    "net/http"
    "github.com/docker/docker/api/types"
    "github.com/gin-gonic/gin"
)


// 修改日志接口，支持实时日志
func getContainerLogs(c *gin.Context) {
    cli, err := docker.NewDockerClient()
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    defer cli.Close()

    id := c.Param("id")
    
    // 先检查容器是否存在
    _, err = cli.ContainerInspect(context.Background(), id)
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

    c.Header("Content-Type", "text/event-stream")
    c.Header("Cache-Control", "no-cache")
    c.Header("Connection", "keep-alive")
    c.Header("X-Accel-Buffering", "no")

    buffer := make([]byte, 8192)
    for {
        n, err := logs.Read(buffer)
        if err != nil {
            break
        }
        if n > 0 {
            // 跳过 Docker log 的头部 8 个字节
            if n > 8 {
                c.Writer.Write(buffer[8:n])
            } else {
                c.Writer.Write(buffer[:n])
            }
            c.Writer.Flush()
        }
    }
}