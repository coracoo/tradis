package system

import (
	"bytes"
	"context"
	"crypto/tls"
	"database/sql"
	"dockerpanel/backend/pkg/database"
	"dockerpanel/backend/pkg/settings"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

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

	go RunNavigationAIBackfill(50)
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
	processContainer(ctr.ID, title, ctr.NetworkSettings.Ports, ctr.Config.Labels, ctr.Config.Image)
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
		processContainer(container.ID, title, container.NetworkSettings.Ports, container.Config.Labels, container.Config.Image)

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

	// 为了复用 processContainer 逻辑，我们需要构造类似 nat.PortMap 的结构，或者直接在这里处理
	// 简单起见，我们重新实现一个针对 types.Container 的处理逻辑，或者只提取第一个公开的 TCP 端口

	s, err := settings.GetSettings()
	if err != nil {
		log.Printf("Failed to get settings for discovery: %v", err)
		return
	}

	publicPorts := make([]int, 0, 4)
	for _, p := range container.Ports {
		if p.Type != "tcp" || p.PublicPort == 0 {
			continue
		}
		publicPorts = append(publicPorts, int(p.PublicPort))
	}
	selected := selectBestPublicPort(publicPorts, s.LanUrl, s.WanUrl, s.AiEnabled)
	if selected == 0 {
		if s.AiEnabled {
			markAutoNavigationDeleted(container.ID)
		}
		return
	}
	registerNavigation(container.ID, title, strconv.Itoa(selected), container.Labels, container.Image, s)
}

func processContainer(containerID, title string, ports nat.PortMap, labels map[string]string, image string) {
	s, err := settings.GetSettings()
	if err != nil {
		log.Printf("Failed to get settings for discovery: %v", err)
		return
	}

	hostPorts := make([]int, 0, 4)
	for portProto, bindings := range ports {
		if !strings.HasSuffix(string(portProto), "/tcp") || len(bindings) == 0 {
			continue
		}
		for _, b := range bindings {
			p, err := strconv.Atoi(strings.TrimSpace(b.HostPort))
			if err != nil || p <= 0 {
				continue
			}
			hostPorts = append(hostPorts, p)
		}
	}

	selected := selectBestPublicPort(hostPorts, s.LanUrl, s.WanUrl, s.AiEnabled)
	if selected == 0 {
		if s.AiEnabled {
			markAutoNavigationDeleted(containerID)
		}
		return
	}
	registerNavigation(containerID, title, strconv.Itoa(selected), labels, image, s)
}

func registerNavigation(containerID, title, publicPort string, labels map[string]string, image string, s settings.Settings) {
	lanBaseUrl := strings.TrimSpace(s.LanUrl)
	wanBaseUrl := strings.TrimSpace(s.WanUrl)

	// 辅助函数：构建 URL
	buildUrl := func(base string, port string) string {
		if base == "" {
			return ""
		}
		u, err := url.Parse(strings.TrimSpace(base))
		if err == nil && u.Scheme != "" && u.Host != "" {
			host := u.Hostname()
			if host == "" {
				return ""
			}
			u.Host = host + ":" + port
			return u.String()
		}
		raw := strings.TrimRight(strings.TrimSpace(base), "/")
		raw = stripTrailingPort(raw)
		if raw == "" {
			return ""
		}
		return fmt.Sprintf("%s:%s", raw, port)
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
		result, err := db.Exec(
			"INSERT INTO navigation_items (title, url, lan_url, wan_url, icon, category, is_auto, container_id) VALUES (?, ?, ?, ?, ?, ?, 1, ?)",
			title, finalUrl, lanUrl, wanUrl, icon, category, containerID,
		)
		if err != nil {
			log.Printf("Failed to auto-register navigation for %s: %v", title, err)
		} else {
			if id, ierr := result.LastInsertId(); ierr == nil {
				existingID = int(id)
			}
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

	if s.AiEnabled && existingID > 0 {
		go func() {
			aiEnrichNavigationItem(existingID, labels, image, s, false)
		}()
	}
}

func removeNavigationForContainer(containerID string) {
	db := database.GetDB()
	_, err := db.Exec("DELETE FROM navigation_items WHERE container_id = ? AND is_auto = 1", containerID)
	if err != nil {
		log.Printf("Failed to remove navigation for container %s: %v", containerID, err)
	}
}

func markAutoNavigationDeleted(containerID string) {
	db := database.GetDB()
	_, _ = db.Exec("UPDATE navigation_items SET is_deleted = 1, updated_at = CURRENT_TIMESTAMP WHERE container_id = ? AND is_auto = 1 AND is_deleted = 0", containerID)
}

func buildTitle(name string, labels map[string]string) string {
	project := strings.TrimSpace(labels["com.docker.compose.project"])
	service := strings.TrimSpace(labels["com.docker.compose.service"])
	if project != "" && service != "" {
		return fmt.Sprintf("%s-%s", project, service)
	}
	return strings.TrimPrefix(name, "/")
}

func selectBestPublicPort(publicPorts []int, lanBaseUrl string, wanBaseUrl string, aiEnabled bool) int {
	dedup := make(map[int]struct{}, len(publicPorts))
	ports := make([]int, 0, len(publicPorts))
	for _, p := range publicPorts {
		if p <= 0 {
			continue
		}
		if _, ok := dedup[p]; ok {
			continue
		}
		dedup[p] = struct{}{}
		ports = append(ports, p)
	}
	if len(ports) == 0 {
		return 0
	}

	sort.Slice(ports, func(i, j int) bool {
		return portScore(ports[i]) > portScore(ports[j])
	})

	if !aiEnabled {
		return ports[0]
	}
	if strings.TrimSpace(lanBaseUrl) == "" && strings.TrimSpace(wanBaseUrl) == "" {
		return ports[0]
	}

	for _, p := range ports {
		if isWebPort(lanBaseUrl, wanBaseUrl, strconv.Itoa(p)) {
			return p
		}
	}
	return 0
}

func portScore(p int) int {
	switch p {
	case 443:
		return 1000
	case 80:
		return 990
	case 8443:
		return 950
	case 8080:
		return 930
	case 8000:
		return 920
	case 3000:
		return 910
	case 9000:
		return 900
	case 9090:
		return 890
	case 5000:
		return 880
	default:
		if p >= 1024 && p <= 65535 {
			return 100
		}
		return 0
	}
}

func isWebPort(lanBaseUrl string, wanBaseUrl string, port string) bool {
	lan := buildHostUrl(lanBaseUrl, port)
	wan := buildHostUrl(wanBaseUrl, port)
	candidates := make([]string, 0, 4)
	if lan != "" {
		candidates = append(candidates, lan)
	}
	if wan != "" && wan != lan {
		candidates = append(candidates, wan)
	}
	if len(candidates) == 0 {
		return false
	}
	for _, u := range candidates {
		if probeHTTP(u) {
			return true
		}
		if strings.HasPrefix(u, "http://") {
			if probeHTTP("https://" + strings.TrimPrefix(u, "http://")) {
				return true
			}
		} else if strings.HasPrefix(u, "https://") {
			if probeHTTP("http://" + strings.TrimPrefix(u, "https://")) {
				return true
			}
		}
	}
	return false
}

func buildHostUrl(baseUrl string, port string) string {
	if strings.TrimSpace(baseUrl) == "" {
		return ""
	}
	u, err := url.Parse(strings.TrimSpace(baseUrl))
	if err != nil || u.Scheme == "" || u.Host == "" {
		raw := strings.TrimRight(strings.TrimSpace(baseUrl), "/")
		raw = stripTrailingPort(raw)
		if raw == "" {
			return ""
		}
		return raw + ":" + port
	}
	host := u.Hostname()
	if host == "" {
		return ""
	}
	u.Host = host + ":" + port
	return u.String()
}

func stripTrailingPort(raw string) string {
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		if u, err := url.Parse(raw); err == nil && u.Host != "" {
			host := u.Hostname()
			if host == "" {
				return raw
			}
			u.Host = host
			u.Path = strings.TrimRight(u.Path, "/")
			return u.String()
		}
	}
	return raw
}

func probeHTTP(target string) bool {
	target = strings.TrimSpace(target)
	if target == "" {
		return false
	}
	client := &http.Client{
		Timeout: 2 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	req, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", "tradis-discovery/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	io.CopyN(io.Discard, resp.Body, 512)
	return true
}

func RunNavigationAIBackfill(limit int) int {
	return RunNavigationAIEnrich(limit, false)
}

func RunNavigationAIEnrichByTitle(title string, limit int, force bool) int {
	s, err := settings.GetSettings()
	if err != nil {
		return 0
	}
	if !s.AiEnabled {
		return 0
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return 0
	}
	if limit <= 0 || limit > 200 {
		limit = 20
	}

	db := database.GetDB()
	query := "SELECT id FROM navigation_items WHERE is_deleted = 0 AND title LIKE ?"
	args := []any{"%" + title + "%"}
	if !force {
		query += " AND ai_generated = 0"
		query += " AND (trim(title) = '' OR trim(category) = '' OR category = '默认' OR trim(icon) = '' OR icon = 'mdi-docker')"
	}
	query += " ORDER BY updated_at DESC LIMIT ?"
	args = append(args, limit)
	rows, err := db.Query(query, args...)
	if err != nil {
		return 0
	}
	defer rows.Close()

	ids := make([]int, 0, limit)
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err == nil && id > 0 {
			ids = append(ids, id)
		}
	}
	for _, id := range ids {
		aiEnrichNavigationItem(id, nil, "", s, force)
	}
	return len(ids)
}

func RunNavigationAIEnrich(limit int, force bool) int {
	s, err := settings.GetSettings()
	if err != nil {
		return 0
	}
	if !s.AiEnabled {
		return 0
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	db := database.GetDB()
	query := "SELECT id FROM navigation_items WHERE is_deleted = 0"
	args := []any{}
	if !force {
		query += " AND ai_generated = 0"
		query += " AND (trim(title) = '' OR trim(category) = '' OR category = '默认' OR trim(icon) = '' OR icon = 'mdi-docker')"
	}
	query += " ORDER BY updated_at DESC LIMIT ?"
	args = append(args, limit)
	rows, err := db.Query(query, args...)
	if err != nil {
		return 0
	}
	defer rows.Close()

	ids := make([]int, 0, limit)
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err == nil && id > 0 {
			ids = append(ids, id)
		}
	}

	for _, id := range ids {
		aiEnrichNavigationItem(id, nil, "", s, force)
	}
	return len(ids)
}

func aiEnrichNavigationItem(navID int, labels map[string]string, image string, s settings.Settings, force bool) {
	apiKey, _ := settings.GetValue("ai_api_key")
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return
	}
	baseUrl := strings.TrimSpace(s.AiBaseUrl)
	model := strings.TrimSpace(s.AiModel)
	if baseUrl == "" || model == "" {
		return
	}

	db := database.GetDB()

	var title sql.NullString
	var lanUrl sql.NullString
	var wanUrl sql.NullString
	var icon sql.NullString
	var category sql.NullString
	var containerID sql.NullString
	var isAuto int
	var isDeleted int
	var aiGenerated int

	if err := db.QueryRow("SELECT title, lan_url, wan_url, icon, category, container_id, is_auto, is_deleted, ai_generated FROM navigation_items WHERE id = ?", navID).
		Scan(&title, &lanUrl, &wanUrl, &icon, &category, &containerID, &isAuto, &isDeleted, &aiGenerated); err != nil {
		return
	}
	if isDeleted == 1 || (aiGenerated == 1 && !force) {
		return
	}

	if !force {
		needFill := strings.TrimSpace(title.String) == "" ||
			strings.TrimSpace(category.String) == "" || strings.TrimSpace(category.String) == "默认" ||
			strings.TrimSpace(icon.String) == "" || strings.TrimSpace(icon.String) == "mdi-docker"
		if !needFill {
			return
		}
	}

	if force {
		appendAiLog("navigation", "info", "ai_enrich_force_mode", map[string]any{"navId": navID})
	}

	if labels == nil {
		labels = map[string]string{}
	}

	currentTitle := strings.TrimSpace(title.String)
	currentCategory := strings.TrimSpace(category.String)
	currentIcon := strings.TrimSpace(icon.String)
	if currentTitle == "" {
		currentTitle = ""
	}
	if currentCategory == "" {
		currentCategory = ""
	}
	if currentIcon == "" {
		currentIcon = ""
	}

	if currentIcon != "" && strings.HasPrefix(currentIcon, "clay:") {
		currentIcon = normalizeAIIconValue(currentIcon)
	}

	if strings.TrimSpace(currentTitle) == "" && strings.TrimSpace(currentCategory) == "" && strings.TrimSpace(currentIcon) == "" && !force {
		return
	}

	labelPairs := make([]string, 0, len(labels))
	for k, v := range labels {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		if strings.HasPrefix(strings.ToLower(k), "com.docker.") {
			labelPairs = append(labelPairs, k+"="+strings.TrimSpace(v))
		}
	}
	sort.Strings(labelPairs)
	if len(labelPairs) > 25 {
		labelPairs = labelPairs[:25]
	}

	userContent := map[string]any{
		"navId":       navID,
		"containerId": strings.TrimSpace(containerID.String),
		"title":       currentTitle,
		"category":    currentCategory,
		"icon":        currentIcon,
		"image":       strings.TrimSpace(image),
		"lanUrl":      strings.TrimSpace(lanUrl.String),
		"wanUrl":      strings.TrimSpace(wanUrl.String),
		"labels":      labelPairs,
		"isAuto":      isAuto == 1,
		"force":       force,
	}
	userBytes, _ := json.Marshal(userContent)

	systemPrompt := strings.TrimSpace(s.AiPrompt)
	if systemPrompt == "" {
		systemPrompt = "你是一个导航整理助手。请输出严格的 JSON，不要输出解释。"
	}
	clayIcons := listClayIconValues(10)
	if len(clayIcons) > 0 {
		systemPrompt += "\n可选 Clay 图标（从中挑一个更贴切的）：\n- " + strings.Join(clayIcons, "\n- ")
	}
	systemPrompt += "\n输出 JSON 格式：{\"title\":\"\",\"category\":\"\",\"icon\":\"\"}。icon 必须是以下之一：mdi-xxx、/icons/clay/<filename>、http(s)://...、/data/pic/...、/uploads/icons/..."
	systemPrompt += "\n不要输出推理过程，不要输出解释，只输出 JSON。"

	payload := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": string(userBytes)},
		},
		"temperature": s.AiTemperature,
		"max_tokens":  800,
	}
	body, _ := json.Marshal(payload)

	endpoint := strings.TrimRight(baseUrl, "/") + "/chat/completions"
	appendAiLog("navigation", "info", "ai_enrich_start", map[string]any{
		"navId":       navID,
		"containerId": strings.TrimSpace(containerID.String),
		"endpoint":    endpoint,
	})

	client := &http.Client{
		Timeout: 25 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		appendAiLog("navigation", "error", "ai_enrich_build_request_failed", map[string]any{"navId": navID})
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		appendAiLog("navigation", "error", "ai_enrich_request_failed", map[string]any{"navId": navID, "error": err.Error(), "latencyMs": time.Since(start).Milliseconds()})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		appendAiLog("navigation", "error", "ai_enrich_bad_status", map[string]any{
			"navId":     navID,
			"status":    resp.Status,
			"latencyMs": time.Since(start).Milliseconds(),
			"body":      strings.TrimSpace(string(snippet)),
		})
		return
	}

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if err != nil {
		appendAiLog("navigation", "error", "ai_enrich_read_failed", map[string]any{"navId": navID})
		return
	}

	var decoded struct {
		Choices []struct {
			Message struct {
				Content          json.RawMessage `json:"content"`
				ReasoningContent json.RawMessage `json:"reasoning_content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		appendAiLog("navigation", "error", "ai_enrich_decode_failed", map[string]any{
			"navId":       navID,
			"respSnippet": compactSnippet(respBody, 2048),
		})
		return
	}
	if len(decoded.Choices) == 0 {
		appendAiLog("navigation", "error", "ai_enrich_empty_choices", map[string]any{
			"navId":       navID,
			"respSnippet": compactSnippet(respBody, 2048),
		})
		return
	}

	content := strings.TrimSpace(extractContentString(decoded.Choices[0].Message.Content))
	if content == "" {
		fallback := strings.TrimSpace(extractContentString(decoded.Choices[0].Message.ReasoningContent))
		if fallback != "" {
			if extracted := extractJSONObjectFromText(fallback); extracted != "" {
				content = extracted
				appendAiLog("navigation", "info", "ai_enrich_fallback_reasoning", map[string]any{"navId": navID})
			}
		}
	}
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)
	if content == "" {
		appendAiLog("navigation", "error", "ai_enrich_empty_content", map[string]any{
			"navId":       navID,
			"respSnippet": compactSnippet(respBody, 2048),
		})
		return
	}

	var out struct {
		Title    string `json:"title"`
		Category string `json:"category"`
		Icon     string `json:"icon"`
	}
	if err := json.Unmarshal([]byte(content), &out); err != nil {
		appendAiLog("navigation", "error", "ai_enrich_parse_failed", map[string]any{
			"navId":   navID,
			"content": content,
		})
		return
	}

	out.Title = strings.TrimSpace(out.Title)
	out.Title = normalizeDuplicatePairTitle(out.Title)
	out.Category = strings.TrimSpace(out.Category)
	out.Icon = normalizeAIIconValue(strings.TrimSpace(out.Icon))
	if out.Icon != "" && !isAllowedNavigationIconValue(out.Icon) {
		out.Icon = ""
	}
	if out.Title == "" && out.Category == "" && out.Icon == "" {
		appendAiLog("navigation", "error", "ai_enrich_empty_result", map[string]any{"navId": navID})
		return
	}

	updateSQL := ""
	if force {
		updateSQL = "UPDATE navigation_items SET title = COALESCE(NULLIF(?, ''), title), category = COALESCE(NULLIF(?, ''), category), icon = COALESCE(NULLIF(?, ''), icon), ai_generated = 1, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND is_deleted = 0"
	} else {
		updateSQL = "UPDATE navigation_items SET title = CASE WHEN trim(title) = '' THEN COALESCE(NULLIF(?, ''), title) ELSE title END, category = CASE WHEN trim(category) = '' OR category = '默认' THEN COALESCE(NULLIF(?, ''), category) ELSE category END, icon = CASE WHEN trim(icon) = '' OR icon = 'mdi-docker' THEN COALESCE(NULLIF(?, ''), icon) ELSE icon END, ai_generated = 1, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND is_deleted = 0 AND ai_generated = 0"
	}
	result, _ := db.Exec(updateSQL, out.Title, out.Category, out.Icon, navID)
	affected := int64(0)
	if result != nil {
		affected, _ = result.RowsAffected()
	}
	appendAiLog("navigation", "info", "ai_enrich_done", map[string]any{
		"navId":     navID,
		"latencyMs": time.Since(start).Milliseconds(),
		"updated":   affected > 0,
		"title":     out.Title,
		"category":  out.Category,
		"icon":      out.Icon,
	})
}

func normalizeDuplicatePairTitle(title string) string {
	t := strings.TrimSpace(title)
	if t == "" {
		return ""
	}
	parts := strings.Split(t, "-")
	if len(parts) == 2 {
		a := strings.TrimSpace(parts[0])
		b := strings.TrimSpace(parts[1])
		if a != "" && a == b {
			return a
		}
	}
	return t
}

func listClayIconValues(limit int) []string {
	dir := ""
	for _, cand := range []string{
		filepath.Join(".", "dist", "icons", "clay"),
		filepath.Join(".", "icons", "clay"),
		filepath.Join("..", "frontend", "public", "icons", "clay"),
	} {
		if st, err := os.Stat(cand); err == nil && st.IsDir() {
			dir = cand
			break
		}
	}
	if dir == "" {
		return nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	items := make([]string, 0, len(entries))
	for _, ent := range entries {
		if ent.IsDir() {
			continue
		}
		name := ent.Name()
		ext := strings.ToLower(filepath.Ext(name))
		switch ext {
		case ".png", ".jpg", ".jpeg", ".webp", ".gif", ".svg", ".ico", ".avif", ".bmp", ".tif", ".tiff":
		default:
			continue
		}
		items = append(items, "/icons/clay/"+name)
	}

	sort.Strings(items)
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items
}

func normalizeAIIconValue(raw string) string {
	v := strings.TrimSpace(raw)
	if v == "" {
		return ""
	}
	if strings.HasPrefix(v, "clay:") {
		name := strings.TrimSpace(strings.TrimPrefix(v, "clay:"))
		if name == "" {
			return ""
		}
		if strings.Contains(name, ".") {
			return "/icons/clay/" + name
		}
		return "/icons/clay/" + name + ".png"
	}
	return v
}

func isAllowedNavigationIconValue(icon string) bool {
	v := strings.TrimSpace(icon)
	if v == "" {
		return false
	}
	if strings.HasPrefix(v, "mdi-") {
		return true
	}
	if strings.HasPrefix(v, "/icons/clay/") {
		return true
	}
	if strings.HasPrefix(v, "/data/pic/") || strings.HasPrefix(v, "/uploads/icons/") {
		return true
	}
	if strings.HasPrefix(v, "http://") || strings.HasPrefix(v, "https://") {
		return true
	}
	return false
}

func appendAiLog(scope string, level string, message string, details map[string]any) {
	if strings.TrimSpace(message) == "" {
		return
	}
	detailStr := ""
	if details != nil {
		if b, err := json.Marshal(details); err == nil {
			detailStr = string(b)
		}
	}
	db := database.GetDB()
	_, _ = db.Exec(
		"INSERT INTO ai_logs (scope, level, message, details) VALUES (?, ?, ?, ?)",
		strings.TrimSpace(scope),
		strings.TrimSpace(level),
		strings.TrimSpace(message),
		detailStr,
	)

	notifyType, notifyMsg, ok := aiLogToNotification(scope, level, message, details)
	if ok {
		_ = database.SaveNotification(&database.Notification{
			Type:    notifyType,
			Message: notifyMsg,
			Read:    false,
		})
	}
}

func aiLogToNotification(scope string, level string, message string, details map[string]any) (string, string, bool) {
	scope = strings.TrimSpace(scope)
	level = strings.TrimSpace(level)
	message = strings.TrimSpace(message)

	if scope != "navigation" {
		return "", "", false
	}

	navID := getIntFromAny(details, "navId")

	if level == "error" {
		msg := "AI 导航识别失败"
		if navID > 0 {
			msg += "（navId=" + strconv.Itoa(navID) + "）"
		}
		msg += "：" + message
		if snippet := getStringFromAny(details, "respSnippet"); snippet != "" {
			msg += " | " + snippet
		}
		if st := getStringFromAny(details, "status"); st != "" {
			msg += " | " + st
		}
		return "error", msg, true
	}

	if message == "ai_enrich_done" && getBoolFromAny(details, "updated") {
		msg := "AI 导航识别完成"
		if navID > 0 {
			msg += "（navId=" + strconv.Itoa(navID) + "）"
		}
		return "success", msg, true
	}

	return "", "", false
}

func getIntFromAny(m map[string]any, key string) int {
	if m == nil {
		return 0
	}
	v, ok := m[key]
	if !ok || v == nil {
		return 0
	}
	switch t := v.(type) {
	case int:
		return t
	case int64:
		return int(t)
	case float64:
		return int(t)
	case string:
		n, _ := strconv.Atoi(strings.TrimSpace(t))
		return n
	default:
		return 0
	}
}

func getBoolFromAny(m map[string]any, key string) bool {
	if m == nil {
		return false
	}
	v, ok := m[key]
	if !ok || v == nil {
		return false
	}
	switch t := v.(type) {
	case bool:
		return t
	case string:
		s := strings.TrimSpace(strings.ToLower(t))
		return s == "true" || s == "1" || s == "yes"
	case float64:
		return t != 0
	default:
		return false
	}
}

func getStringFromAny(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	default:
		b, err := json.Marshal(t)
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(b))
	}
}

func compactSnippet(b []byte, limit int) string {
	if limit <= 0 {
		limit = 2048
	}
	if len(b) > limit {
		b = b[:limit]
	}
	s := strings.TrimSpace(string(b))
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	if len(s) > 512 {
		s = s[:512]
	}
	return s
}

func extractContentString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	if string(raw) == "null" {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &parts); err == nil {
		buf := strings.Builder{}
		for _, p := range parts {
			if strings.TrimSpace(p.Text) == "" {
				continue
			}
			if buf.Len() > 0 {
				buf.WriteString("\n")
			}
			buf.WriteString(p.Text)
		}
		return buf.String()
	}
	return ""
}

func extractJSONObjectFromText(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start < 0 || end < 0 || end <= start {
		return ""
	}
	cand := strings.TrimSpace(s[start : end+1])
	var tmp any
	if json.Unmarshal([]byte(cand), &tmp) == nil {
		return cand
	}
	return ""
}
