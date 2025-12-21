package system

import (
	"context"
	"database/sql"
	"dockerpanel/backend/pkg/database"
	"dockerpanel/backend/pkg/settings"
	"fmt"
	"log"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

// ProcessContainerDiscovery 扫描现有容器并注册导航项
func ProcessContainerDiscovery() {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Printf("Error creating docker client for discovery: %v", err)
		return
	}
	defer cli.Close()

	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{All: true})
	if err != nil {
		log.Printf("Error listing containers for discovery: %v", err)
		return
	}

	for _, container := range containers {
		updateNavigationForContainer(container)
	}
}

// RebuildAutoNavigationAll 清空并重新生成所有自动发现的导航项（不会影响手动添加的导航项）。
func RebuildAutoNavigationAll() {
	db := database.GetDB()

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Printf("Error creating docker client for discovery rebuild: %v", err)
		return
	}
	defer cli.Close()

	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{All: true})
	if err != nil {
		log.Printf("Error listing containers for discovery rebuild: %v", err)
		return
	}

	existingContainerIDs := make(map[string]struct{}, len(containers))
	for _, ctr := range containers {
		existingContainerIDs[ctr.ID] = struct{}{}
	}

	rows, err := db.Query("SELECT id, container_id FROM navigation_items WHERE is_auto = 1")
	if err == nil {
		defer rows.Close()

		for rows.Next() {
			var id int
			var containerID sql.NullString
			if scanErr := rows.Scan(&id, &containerID); scanErr != nil {
				continue
			}
			if !containerID.Valid || strings.TrimSpace(containerID.String) == "" {
				_, _ = db.Exec("DELETE FROM navigation_items WHERE id = ?", id)
				continue
			}
			if _, ok := existingContainerIDs[containerID.String]; !ok {
				_, _ = db.Exec("DELETE FROM navigation_items WHERE id = ?", id)
			}
		}
	}

	ProcessContainerDiscovery()
}

// RebuildAutoNavigationForContainer 仅针对指定容器重建自动导航项（容器不存在则仅清理）。
func RebuildAutoNavigationForContainer(containerID string) {
	containerID = strings.TrimSpace(containerID)
	if containerID == "" {
		return
	}

	db := database.GetDB()
	_, _ = db.Exec("DELETE FROM navigation_items WHERE container_id = ? AND is_auto = 1", containerID)

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Printf("Error creating docker client for container rebuild: %v", err)
		return
	}
	defer cli.Close()

	ctr, err := cli.ContainerInspect(context.Background(), containerID)
	if err != nil {
		return
	}

	title := buildTitle(ctr.Name, ctr.Config.Labels)
	processContainer(ctr.ID, title, ctr.NetworkSettings.Ports)
}

// RebuildAutoNavigationForComposeProject 仅针对指定 Compose 项目重建自动导航项（不会影响其他项目）。
func RebuildAutoNavigationForComposeProject(projectName string) {
	projectName = strings.TrimSpace(projectName)
	if projectName == "" {
		return
	}

	db := database.GetDB()
	_, _ = db.Exec("DELETE FROM navigation_items WHERE is_auto = 1 AND title LIKE ?", projectName+"-%")

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Printf("Error creating docker client for project rebuild: %v", err)
		return
	}
	defer cli.Close()

	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{
		All: true,
		Filters: filters.NewArgs(
			filters.Arg("label", "com.docker.compose.project="+projectName),
		),
	})
	if err != nil {
		log.Printf("Error listing containers for project rebuild (%s): %v", projectName, err)
		return
	}

	for _, ctr := range containers {
		_, _ = db.Exec("DELETE FROM navigation_items WHERE container_id = ? AND is_auto = 1", ctr.ID)
		updateNavigationForContainer(ctr)
	}
}

// WatchContainerEvents 监听容器事件以更新导航项
func WatchContainerEvents() {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Printf("Error creating docker client for event watching: %v", err)
		return
	}

	msgs, errs := cli.Events(context.Background(), types.EventsOptions{
		Filters: filters.NewArgs(
			filters.Arg("type", "container"),
			filters.Arg("event", "start"),
			filters.Arg("event", "destroy"),
			filters.Arg("event", "die"),
			filters.Arg("event", "rename"),
		),
	})

	go func() {
		for {
			select {
			case event := <-msgs:
				handleContainerEvent(cli, event)
			case err := <-errs:
				if err != nil {
					log.Printf("Error reading docker events: %v", err)
					return
				}
			}
		}
	}()
}

func handleContainerEvent(cli *client.Client, event events.Message) {
	switch event.Action {
	case "start", "rename":
		// Inspect container to get details
		container, err := cli.ContainerInspect(context.Background(), event.Actor.ID)
		if err != nil {
			log.Printf("Error inspecting container %s: %v", event.Actor.ID, err)
			return
		}

		// 处理容器端口映射和导航注册
		title := buildTitle(container.Name, container.Config.Labels)
		processContainer(container.ID, title, container.NetworkSettings.Ports)

	case "destroy":
		// Remove navigation item if it was auto-created for this container
		removeNavigationForContainer(event.Actor.ID)
		if tx, err := database.GetDB().Begin(); err == nil {
			if derr := database.DeleteReservedPortsByOwnerTx(tx, event.Actor.ID); derr != nil {
				_ = tx.Rollback()
			} else {
				_ = tx.Commit()
			}
		}

	case "die":
		removeNavigationForContainer(event.Actor.ID)
	}
}

// updateNavigationForContainer 检查容器标签并更新导航表
func updateNavigationForContainer(container types.Container) {
	// 容器名称通常以 / 开头，去除它
	name := strings.TrimPrefix(container.Names[0], "/")
	title := buildTitle(name, container.Labels)

	// 从 Ports 中提取映射信息
	// ContainerList 返回的 Ports 结构与 Inspect 不同，需要转换或直接使用
	// types.Port: IP, PrivatePort, PublicPort, Type

	portsMap := make(map[string][]string)
	for _, p := range container.Ports {
		if p.PublicPort != 0 {
			portKey := fmt.Sprintf("%d/%s", p.PrivatePort, p.Type)
			// 我们只关心 TCP 端口
			if p.Type == "tcp" {
				// 将 uint16 转换为 string
				publicPortStr := fmt.Sprintf("%d", p.PublicPort)
				portsMap[portKey] = append(portsMap[portKey], publicPortStr)
			}
		}
	}

	// 为了复用 processContainer 逻辑，我们需要构造类似 nat.PortMap 的结构，或者直接在这里处理
	// 简单起见，我们重新实现一个针对 types.Container 的处理逻辑，或者只提取第一个公开的 TCP 端口

	var firstPublicPort string
	// 优先查找 80, 443, 8080, 3000 等常用端口
	// 但用户的要求是：只要有 TCP 端口映射，就自动发现

	// 寻找任意一个映射出的 TCP 端口
	for _, p := range container.Ports {
		if p.Type == "tcp" && p.PublicPort != 0 {
			firstPublicPort = fmt.Sprintf("%d", p.PublicPort)
			break
		}
	}

	if firstPublicPort != "" {
		registerNavigation(container.ID, title, firstPublicPort)
	}
}

func processContainer(containerID, title string, ports nat.PortMap) { // nat.PortMap 简化表示

	var firstPublicPort string

	// 遍历端口映射，找到第一个映射到主机的 TCP 端口
	// ports map key is "port/proto" e.g. "80/tcp"
	for portProto, bindings := range ports {
		if strings.HasSuffix(string(portProto), "/tcp") && len(bindings) > 0 {
			firstPublicPort = bindings[0].HostPort
			break
		}
	}

	if firstPublicPort != "" {
		registerNavigation(containerID, title, firstPublicPort)
	}
}

func registerNavigation(containerID, title, publicPort string) {
	// 获取全局配置的 LanUrl 和 WanUrl
	lanBaseUrl := settings.GetLanUrl()
	wanBaseUrl := settings.GetWanUrl()

	// 辅助函数：构建 URL
	buildUrl := func(base string, port string) string {
		if base == "" {
			return ""
		}
		// 去除可能存在的端口
		if parts := strings.Split(base, ":"); len(parts) > 2 {
			base = strings.Join(parts[:2], ":")
		}
		return fmt.Sprintf("%s:%s", base, port)
	}

	lanUrl := buildUrl(lanBaseUrl, publicPort)
	wanUrl := buildUrl(wanBaseUrl, publicPort)

	// 兼容旧的 url 字段，优先使用 lanUrl
	finalUrl := lanUrl
	if finalUrl == "" {
		finalUrl = wanUrl
	}

	// 写入数据库
	db := database.GetDB()

	// 检查是否已存在
	var existingID int
	err := db.QueryRow("SELECT id FROM navigation_items WHERE container_id = ?", containerID).Scan(&existingID)

	icon := "mdi-docker" // 默认图标
	category := "默认"
	if strings.TrimSpace(title) == "" {
		title = containerID
	}

	if err == sql.ErrNoRows {
		// Create new
		_, err = db.Exec(
			"INSERT INTO navigation_items (title, url, lan_url, wan_url, icon, category, is_auto, container_id) VALUES (?, ?, ?, ?, ?, ?, 1, ?)",
			title, finalUrl, lanUrl, wanUrl, icon, category, containerID,
		)
		if err != nil {
			log.Printf("Failed to auto-register navigation for %s: %v", title, err)
		} else {
			log.Printf("Auto-registered navigation for %s -> LAN: %s, WAN: %s", title, lanUrl, wanUrl)
		}
	} else if err == nil {
		// Update existing
		_, err = db.Exec(
			"UPDATE navigation_items SET title = ?, url = ?, lan_url = ?, wan_url = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
			title, finalUrl, lanUrl, wanUrl, existingID,
		)
		if err != nil {
			log.Printf("Failed to update navigation for %s: %v", title, err)
		}
	}
}

func removeNavigationForContainer(containerID string) {
	db := database.GetDB()
	_, err := db.Exec("DELETE FROM navigation_items WHERE container_id = ? AND is_auto = 1", containerID)
	if err != nil {
		log.Printf("Failed to remove navigation for container %s: %v", containerID, err)
	}
}

func buildTitle(name string, labels map[string]string) string {
	project := strings.TrimSpace(labels["com.docker.compose.project"])
	service := strings.TrimSpace(labels["com.docker.compose.service"])
	if project != "" && service != "" {
		return fmt.Sprintf("%s-%s", project, service)
	}
	return strings.TrimPrefix(name, "/")
}
