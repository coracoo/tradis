package api

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"

	"github.com/creack/pty"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// containerTerminalCLI 使用 Docker CLI 通过 docker.sock 建立交互式终端，并通过 WebSocket 与前端桥接
// 路径：GET /api/containers/:id/terminal
// 协议：前端需发送三类消息
// 1) {type:"command", data:"/bin/sh"} 用于指定初次命令（默认 /bin/sh）
// 2) {type:"input", data:"..."} 用户输入写入到容器
// 3) {type:"resize", data:"{\"rows\":24,\"cols\":80}"} 调整终端大小
func containerTerminalCLI(c *gin.Context) {
	containerId := c.Param("id")

	// 升级连接为 WebSocket
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		fmt.Printf("WebSocket升级失败: %v\n", err)
		c.JSON(500, gin.H{"error": fmt.Sprintf("WebSocket升级失败: %v", err)})
		return
	}
	defer ws.Close()

	// 默认命令
	cmdToUse := []string{"/bin/sh"}

	// 尝试读取前端初始命令
	if messageType, p, readErr := ws.ReadMessage(); readErr == nil && messageType == websocket.TextMessage {
		var msg struct {
			Type string `json:"type"`
			Data string `json:"data"`
		}
		if unmarshalErr := json.Unmarshal(p, &msg); unmarshalErr == nil && msg.Type == "command" && msg.Data != "" {
			cmdToUse = parseCommand(msg.Data)
		}
	}

	// 构造 docker exec -it 命令
	args := append([]string{"exec", "-it", containerId}, cmdToUse...)
	cmd := exec.Command("docker", args...)
	// 显式指定通过本机 docker.sock
	cmd.Env = append(os.Environ(), "DOCKER_HOST=unix:///var/run/docker.sock")

	// 使用 pty 以获得交互式 TTY
	ptmx, err := pty.Start(cmd)
	if err != nil {
		writeWSErr(ws, fmt.Sprintf("启动终端失败: %v", err))
		return
	}
	defer func() { _ = ptmx.Close() }()

	// 用于安全写 WS
	var wsWriteMu sync.Mutex
	done := make(chan struct{})

	// 容器输出 -> WebSocket 二进制
	go func() {
		defer func() {
			close(done)
		}()
		buf := make([]byte, 4096)
		for {
			n, err := ptmx.Read(buf)
			if err != nil {
				if err != io.EOF {
					wsWriteMu.Lock()
					_ = ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("读取容器输出错误: %v", err)))
					wsWriteMu.Unlock()
				}
				return
			}
			if n > 0 {
				wsWriteMu.Lock()
				err = ws.WriteMessage(websocket.BinaryMessage, buf[:n])
				wsWriteMu.Unlock()
				if err != nil {
					return
				}
			}
		}
	}()

	// WebSocket 输入与控制 -> 容器输入/终端尺寸
	go func() {
		defer func() {
			select {
			case <-done:
			default:
				close(done)
			}
		}()
		for {
			messageType, p, err := ws.ReadMessage()
			if err != nil {
				break
			}
			if messageType != websocket.TextMessage {
				continue
			}
			var msg struct {
				Type string `json:"type"`
				Data string `json:"data"`
			}
			if err := json.Unmarshal(p, &msg); err != nil {
				continue
			}
			switch msg.Type {
			case "input":
				if _, err := ptmx.Write([]byte(msg.Data)); err != nil {
					wsWriteMu.Lock()
					_ = ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("写入容器输入错误: %v", err)))
					wsWriteMu.Unlock()
				}
			case "resize":
				var size struct {
					Rows uint16 `json:"rows"`
					Cols uint16 `json:"cols"`
				}
				if err := json.Unmarshal([]byte(msg.Data), &size); err == nil {
					_ = pty.Setsize(ptmx, &pty.Winsize{Rows: size.Rows, Cols: size.Cols})
				}
			}
		}
	}()

	<-done
}

// writeWSErr 统一 WebSocket 错误输出
func writeWSErr(ws *websocket.Conn, msg string) {
	_ = ws.WriteMessage(websocket.TextMessage, []byte(msg))
}

// parseCommand 将用户输入拆分为命令与参数切片
func parseCommand(s string) []string {
	// 简化实现：按空格拆分，保留顺序
	out := []string{}
	cur := ""
	inQuote := false
	var quote byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case ' ', '\t':
			if inQuote {
				cur += string(c)
			} else {
				if cur != "" {
					out = append(out, cur)
					cur = ""
				}
			}
		case '\'', '"':
			if inQuote && c == quote {
				inQuote = false
			} else if !inQuote {
				inQuote = true
				quote = c
			} else {
				cur += string(c)
			}
		default:
			cur += string(c)
		}
	}
	if cur != "" {
		out = append(out, cur)
	}
	if len(out) == 0 {
		return []string{"/bin/sh"}
	}
	return out
}
