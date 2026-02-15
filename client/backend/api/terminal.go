package api

import (
	"context"
	"dockerpanel/backend/pkg/docker"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type wsMessage struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

func parseWsMessage(payload []byte) (wsMessage, bool) {
	var msg wsMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		return msg, false
	}
	if strings.TrimSpace(msg.Type) == "" {
		return msg, false
	}
	return msg, true
}

func parseStringPayload(raw json.RawMessage) (string, bool) {
	if len(raw) == 0 {
		return "", false
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", false
	}
	return s, true
}

func parseResizePayload(raw json.RawMessage) (uint, uint, bool) {
	if len(raw) == 0 {
		return 0, 0, false
	}
	var size struct {
		Rows uint `json:"rows"`
		Cols uint `json:"cols"`
	}
	if err := json.Unmarshal(raw, &size); err == nil {
		return size.Rows, size.Cols, true
	}
	var asString string
	if err := json.Unmarshal(raw, &asString); err != nil {
		return 0, 0, false
	}
	if err := json.Unmarshal([]byte(asString), &size); err != nil {
		return 0, 0, false
	}
	return size.Rows, size.Cols, true
}

// 添加一个新的终端处理函数，使用Docker SDK直接执行命令
func containerExec(c *gin.Context) {
	containerId := c.Param("id")
	if forbidIfSelfContainer(c, containerId) {
		return
	}
	command := c.Query("cmd")

	if command == "" {
		command = "/bin/sh" // 默认命令
	}

	fmt.Printf("执行容器命令: %s, 容器ID: %s\n", command, containerId)

	cli, ok := getDockerClient(c)
	if !ok {
		return
	}
	defer cli.Close()

	// 检查容器是否存在并运行
	containerInfo, err := cli.ContainerInspect(context.Background(), containerId)
	if err != nil {
		respondError(c, http.StatusNotFound, "容器不存在", err)
		return
	}

	if !containerInfo.State.Running {
		respondError(c, http.StatusBadRequest, "容器未运行，无法执行命令", nil)
		return
	}

	// 解析命令
	cmdParts := strings.Fields(command)

	// 容器执行命令的配置
	execConfig := types.ExecConfig{
		Cmd:          cmdParts,
		AttachStdout: true,
		AttachStderr: true,
		AttachStdin:  false, // 不需要输入
		Tty:          false, // 不使用TTY
	}

	// 创建容器执行命令
	execResp, err := cli.ContainerExecCreate(context.Background(), containerId, execConfig)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "创建exec命令失败", err)
		return
	}

	// 执行容器命令并获取输出
	resp, err := cli.ContainerExecAttach(context.Background(), execResp.ID, types.ExecStartCheck{})
	if err != nil {
		respondError(c, http.StatusInternalServerError, "附加到exec命令失败", err)
		return
	}
	defer resp.Close()

	// 读取所有输出
	output, err := io.ReadAll(resp.Reader)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "读取命令输出失败", err)
		return
	}

	// 返回命令输出
	c.JSON(http.StatusOK, gin.H{
		"output":       string(output),
		"command":      command,
		"container_id": containerId,
	})
}

// 定义WebSocket升级器
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许所有来源的WebSocket连接
	},
}

// 添加WebSocket终端处理函数
// containerTerminal 建立与容器的交互式终端会话（WebSocket <-> Docker Exec TTY）
// 行为：
// 1) 升级为 WebSocket；在握手阶段读取前端发送的 entrypoint 命令（type=command）作为 shell/入口命令；
// 2) 创建 Docker Exec（TTY=true），桥接容器的输入输出到 WebSocket；
// 3) 支持窗口尺寸调整（type=resize）与持续输入（type=input），保持会话交互直至任一端关闭。
func containerTerminal(c *gin.Context) {
	containerId := c.Param("id")
	if forbidIfSelfContainer(c, containerId) {
		return
	}

	fmt.Printf("收到终端连接请求: %s\n", c.Param("id"))

	// 升级HTTP连接为WebSocket
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		fmt.Printf("WebSocket升级失败: %v\n", err)
		respondError(c, http.StatusInternalServerError, "WebSocket升级失败", err)
		return
	}
	defer ws.Close()

	// 不向TTY写入成功提示，仅在服务端记录
	fmt.Println("WebSocket连接已建立，准备附加容器终端...")

	cli, err := docker.NewDockerClient()
	if err != nil {
		errMsg := fmt.Sprintf("Docker客户端创建失败: %v\n", err)
		fmt.Println(errMsg)
		ws.WriteMessage(websocket.TextMessage, []byte(errMsg))
		return
	}
	defer cli.Close()

	// 检查容器是否存在
	_, err = cli.ContainerInspect(context.Background(), containerId)
	if err != nil {
		errMsg := fmt.Sprintf("容器不存在或无法访问: %v\n", err)
		fmt.Println(errMsg)
		ws.WriteMessage(websocket.TextMessage, []byte(errMsg))
		return
	}

	// 入口命令默认值，可被握手阶段的前端 'command' 消息覆盖
	shellCmd := []string{"/bin/sh"}
	if cmdParam := c.Query("cmd"); cmdParam != "" {
		shellCmd = strings.Fields(cmdParam)
	}

	// 尝试在握手阶段读取一次前端发来的入口命令
	// 前端在 onopen 立即发送 {type:"command", data:"/bin/bash"}
	ws.SetReadDeadline(time.Now().Add(800 * time.Millisecond))
	if mt, p, err := ws.ReadMessage(); err == nil && mt == websocket.TextMessage {
		if msg, ok := parseWsMessage(p); ok && msg.Type == "command" {
			if cmd, ok := parseStringPayload(msg.Data); ok && strings.TrimSpace(cmd) != "" {
				shellCmd = strings.Fields(cmd)
				fmt.Printf("使用入口命令: %s\n", strings.Join(shellCmd, " "))
			}
		}
	}
	// 取消读取超时，进入常规会话流程
	ws.SetReadDeadline(time.Time{})

	// 创建exec配置（TTY 模式）并显式设置 UTF-8 相关环境，确保中文显示正确
	execConfig := types.ExecConfig{
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true,
		Cmd:          shellCmd,
		Env: []string{
			"LANG=C.UTF-8",
			"LC_ALL=C.UTF-8",
			"TERM=xterm-256color",
		},
	}

	fmt.Printf("为容器 %s 创建exec实例\n", containerId)
	// 创建exec实例
	execResp, err := cli.ContainerExecCreate(context.Background(), containerId, execConfig)
	if err != nil {
		errMsg := fmt.Sprintf("创建exec实例失败: %v\r\n", err)
		fmt.Println(errMsg)
		ws.WriteMessage(websocket.TextMessage, []byte(errMsg))
		return
	}

	fmt.Printf("附加到exec实例 %s\n", execResp.ID)
	// 附加到exec实例
	hijacked, err := cli.ContainerExecAttach(context.Background(), execResp.ID, types.ExecStartCheck{
		Detach: false,
		Tty:    true,
	})
	if err != nil {
		errMsg := fmt.Sprintf("附加到exec实例失败: %v\r\n", err)
		fmt.Println(errMsg)
		ws.WriteMessage(websocket.TextMessage, []byte(errMsg))
		return
	}
	defer hijacked.Close()

	fmt.Println("成功附加到容器，开始数据传输")

	// 处理WebSocket消息
	// 使用互斥锁确保WebSocket写入的线程安全
	var wsWriteMu sync.Mutex

	// 创建一个完成通道，用于同步goroutine（通过 sync.Once 保证只关闭一次）
	done := make(chan struct{})
	var closeOnce sync.Once
	closeDone := func() { closeOnce.Do(func() { close(done) }) }

	// 从容器输出读取并发送到WebSocket
	go func() {
		defer func() {
			fmt.Println("容器输出处理goroutine结束")
			closeDone()
		}()

		buf := make([]byte, 4096)
		for {
			nr, err := hijacked.Reader.Read(buf)
			if err != nil {
				if err != io.EOF {
					wsWriteMu.Lock()
					fmt.Printf("读取容器输出错误: %v\n", err)
					ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("读取容器输出错误: %v\r\n", err)))
					wsWriteMu.Unlock()
				}
				break
			}

			if nr > 0 {
				wsWriteMu.Lock()
				err = ws.WriteMessage(websocket.BinaryMessage, buf[:nr])
				wsWriteMu.Unlock()
				if err != nil {
					fmt.Printf("发送WebSocket消息错误: %v\n", err)
					break
				}
			}
		}
	}()

	// 从WebSocket读取并写入容器输入
	go func() {
		defer func() {
			fmt.Println("WebSocket输入处理goroutine结束")
			// 通知另一个goroutine结束（只关闭一次，避免 panic: close of closed channel）
			closeDone()
		}()

		for {
			messageType, p, err := ws.ReadMessage()
			if err != nil {
				fmt.Printf("读取WebSocket消息错误: %v\n", err)
				break
			}

			if messageType == websocket.TextMessage {
				msg, ok := parseWsMessage(p)
				if !ok {
					fmt.Printf("解析WebSocket消息错误\n")
					continue
				}
				fmt.Printf("收到WebSocket消息: type=%s, data长度=%d\n", msg.Type, len(msg.Data))

				switch msg.Type {
				case "input":
					input, ok := parseStringPayload(msg.Data)
					if !ok {
						fmt.Printf("解析输入数据错误\n")
						continue
					}
					_, err = hijacked.Conn.Write([]byte(input))
					if err != nil {
						wsWriteMu.Lock()
						fmt.Printf("写入容器输入错误: %v\n", err)
						ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("写入容器输入错误: %v\r\n", err)))
						wsWriteMu.Unlock()
						break
					}
				case "resize":
					rows, cols, ok := parseResizePayload(msg.Data)
					if !ok {
						fmt.Printf("解析终端大小数据错误\n")
						continue
					}
					fmt.Printf("调整终端大小: rows=%d, cols=%d\n", rows, cols)
					err = cli.ContainerExecResize(context.Background(), execResp.ID, types.ResizeOptions{
						Height: rows,
						Width:  cols,
					})
					if err != nil {
						wsWriteMu.Lock()
						fmt.Printf("调整终端大小错误: %v\n", err)
						ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("调整终端大小错误: %v\r\n", err)))
						wsWriteMu.Unlock()
					}
				case "command":
					continue
				}
			} else {
				fmt.Printf("收到非文本消息: type=%d, 长度=%d\n", messageType, len(p))
			}
		}
	}()

	// 等待任一goroutine完成
	<-done
	fmt.Println("终端会话结束")
}

// 修改路由注册函数，添加替代方法的路由
func RegisterTerminalRoutes(r *gin.RouterGroup) {
	// 仅注册非交互式命令执行路由，终端路由由 RegisterContainerRoutes 提供
	r.GET("/containers/:id/exec", containerExec)
}
