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
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

var navAIEnrichLocks sync.Map
var navAIEnrichBatchMu sync.Mutex
var navAIEnrichBatchRunning atomic.Bool
var autoAIEnrichSuppressedUntil atomic.Int64

func suppressAutoAIEnrichFor(d time.Duration) {
	if d <= 0 {
		return
	}
	autoAIEnrichSuppressedUntil.Store(time.Now().Add(d).UnixNano())
}

func isAutoAIEnrichSuppressed() bool {
	return time.Now().UnixNano() < autoAIEnrichSuppressedUntil.Load()
}

func withNavAIEnrichLock(navID int, fn func()) {
	if navID <= 0 || fn == nil {
		return
	}
	muAny, _ := navAIEnrichLocks.LoadOrStore(navID, &sync.Mutex{})
	mu := muAny.(*sync.Mutex)
	mu.Lock()
	defer mu.Unlock()
	fn()
}

func cleanupOrphanAutoNavigation(existingContainerIDs map[string]struct{}) {
	db := database.GetDB()
	rows, err := db.Query("SELECT id, container_id FROM navigation_items WHERE is_auto = 1")
	if err != nil {
		return
	}
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

func CleanupOrphanAutoNavigationNow() {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return
	}
	defer cli.Close()

	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{All: true})
	if err != nil {
		return
	}
	existingContainerIDs := make(map[string]struct{}, len(containers))
	for _, ctr := range containers {
		existingContainerIDs[ctr.ID] = struct{}{}
	}
	cleanupOrphanAutoNavigation(existingContainerIDs)
}

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

	existingContainerIDs := make(map[string]struct{}, len(containers))
	for _, ctr := range containers {
		existingContainerIDs[ctr.ID] = struct{}{}
	}
	cleanupOrphanAutoNavigation(existingContainerIDs)

	for _, container := range containers {
		updateNavigationForContainer(container)
	}
}

// RebuildAutoNavigationAll 清空并重新生成所有自动发现的导航项（不会影响手动添加的导航项）。
func RebuildAutoNavigationAll() {
	suppressAutoAIEnrichFor(20 * time.Second)
	db := database.GetDB()
	_, _ = db.Exec("DELETE FROM navigation_items WHERE is_auto = 1")

	ProcessContainerDiscovery()
}

// RebuildAutoNavigationForContainer 仅针对指定容器重建自动导航项（容器不存在则仅清理）。
func RebuildAutoNavigationForContainer(containerID string) {
	containerID = strings.TrimSpace(containerID)
	if containerID == "" {
		return
	}
	suppressAutoAIEnrichFor(20 * time.Second)

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
	suppressAutoAIEnrichFor(20 * time.Second)

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
		return
	}
}

func shouldProbeWebPorts(s settings.Settings) bool {
	if !s.AiEnabled {
		return false
	}
	return strings.TrimSpace(s.LanUrl) != "" || strings.TrimSpace(s.WanUrl) != ""
}

func resolveAndRegisterNavigation(containerID string, title string, ports []int, labels map[string]string, image string, s settings.Settings) {
	selected := selectBestPublicPort(ports, s.LanUrl, s.WanUrl, s.AiEnabled)
	if selected != 0 {
		registerNavigation(containerID, title, strconv.Itoa(selected), labels, image, s)
		return
	}
	markAutoNavigationDeleted(containerID)
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
	resolveAndRegisterNavigation(container.ID, title, publicPorts, container.Labels, container.Image, s)
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

	resolveAndRegisterNavigation(containerID, title, hostPorts, labels, image, s)
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

	var existingID int
	var extraIDs []int
	rows, err := db.Query("SELECT id FROM navigation_items WHERE container_id = ? AND is_auto = 1 ORDER BY id ASC", containerID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id int
			if scanErr := rows.Scan(&id); scanErr != nil {
				continue
			}
			if existingID == 0 {
				existingID = id
			} else {
				extraIDs = append(extraIDs, id)
			}
		}
	}
	for _, id := range extraIDs {
		_, _ = db.Exec("DELETE FROM navigation_items WHERE id = ?", id)
	}

	icon := "mdi-docker" // 默认图标
	category := "未分类"
	if strings.TrimSpace(title) == "" {
		title = containerID
	}

	created := false
	if existingID == 0 {
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
			created = existingID > 0
			log.Printf("Auto-registered navigation for %s -> LAN: %s, WAN: %s", title, lanUrl, wanUrl)
		}
	} else {
		// Update existing
		_, err = db.Exec(
			"UPDATE navigation_items SET title = ?, url = ?, lan_url = ?, wan_url = ?, is_deleted = 0, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
			title, finalUrl, lanUrl, wanUrl, existingID,
		)
		if err != nil {
			log.Printf("Failed to update navigation for %s: %v", title, err)
		}
	}

	if created && s.AiEnabled && existingID > 0 && !isAutoAIEnrichSuppressed() && !navAIEnrichBatchRunning.Load() {
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
	return ports[0]
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
		if probeWeb(u) {
			return true
		}
		if strings.HasPrefix(u, "http://") {
			if probeWeb("https://" + strings.TrimPrefix(u, "http://")) {
				return true
			}
		} else if strings.HasPrefix(u, "https://") {
			if probeWeb("http://" + strings.TrimPrefix(u, "https://")) {
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

func probeWeb(target string) bool {
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

	req, err := http.NewRequest(http.MethodHead, target, nil)
	if err == nil {
		req.Header.Set("User-Agent", "tradis-discovery/1.0")
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/json;q=0.9,*/*;q=0.8")
		resp, err := client.Do(req)
		if err == nil {
			io.CopyN(io.Discard, resp.Body, 256)
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 400 {
				return true
			}
			if resp.StatusCode == http.StatusMethodNotAllowed {
				// fallthrough to GET
			} else {
				return false
			}
		}
	}

	getReq, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		return false
	}
	getReq.Header.Set("User-Agent", "tradis-discovery/1.0")
	getReq.Header.Set("Accept", "text/html,application/xhtml+xml,application/json;q=0.9,*/*;q=0.8")
	resp, err := client.Do(getReq)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		io.CopyN(io.Discard, resp.Body, 256)
		return false
	}
	ct := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Type")))
	snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	body := strings.TrimSpace(string(snippet))
	if strings.HasPrefix(body, "{") {
		lower := strings.ToLower(body)
		if strings.Contains(ct, "application/json") && (strings.Contains(lower, "\"detail\"") && strings.Contains(lower, "not found")) {
			return false
		}
	}
	return true
}

func probeImage(target string) bool {
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
	req.Header.Set("Accept", "image/*,application/octet-stream;q=0.9,*/*;q=0.8")
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		io.CopyN(io.Discard, resp.Body, 256)
		return false
	}

	ct := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Type")))
	snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	snippet = bytes.TrimSpace(snippet)
	lowerText := strings.ToLower(string(snippet))
	if strings.Contains(ct, "text/html") || strings.Contains(ct, "application/json") {
		return false
	}
	if strings.HasPrefix(lowerText, "{") {
		if strings.Contains(lowerText, "\"detail\"") || strings.Contains(lowerText, "\"error\"") || strings.Contains(lowerText, "not found") {
			return false
		}
	}
	if strings.Contains(lowerText, "<!doctype") || strings.Contains(lowerText, "<html") || strings.Contains(lowerText, "<head") || strings.Contains(lowerText, "not found") || strings.Contains(lowerText, "404") {
		return false
	}

	isICO := func(b []byte) bool {
		return len(b) >= 4 && b[0] == 0x00 && b[1] == 0x00 && b[2] == 0x01 && b[3] == 0x00
	}
	isPNG := func(b []byte) bool {
		return len(b) >= 8 && bytes.Equal(b[:8], []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a})
	}
	isJPG := func(b []byte) bool {
		return len(b) >= 3 && b[0] == 0xff && b[1] == 0xd8 && b[2] == 0xff
	}
	isGIF := func(b []byte) bool {
		return len(b) >= 6 && (bytes.Equal(b[:6], []byte("GIF87a")) || bytes.Equal(b[:6], []byte("GIF89a")))
	}
	isWEBP := func(b []byte) bool {
		return len(b) >= 12 && bytes.Equal(b[:4], []byte("RIFF")) && bytes.Equal(b[8:12], []byte("WEBP"))
	}
	isSVG := func(text string) bool {
		return strings.Contains(text, "<svg")
	}

	if strings.Contains(ct, "image/svg") {
		return isSVG(lowerText)
	}
	if strings.Contains(ct, "image/png") {
		return isPNG(snippet)
	}
	if strings.Contains(ct, "image/jpeg") || strings.Contains(ct, "image/jpg") {
		return isJPG(snippet)
	}
	if strings.Contains(ct, "image/gif") {
		return isGIF(snippet)
	}
	if strings.Contains(ct, "image/webp") {
		return isWEBP(snippet)
	}

	if strings.Contains(ct, "image/x-icon") || strings.Contains(ct, "image/vnd.microsoft.icon") {
		return isICO(snippet)
	}

	if strings.HasPrefix(ct, "image/") {
		return len(snippet) > 0
	}

	if strings.Contains(ct, "application/octet-stream") {
		if isICO(snippet) || isPNG(snippet) || isJPG(snippet) || isGIF(snippet) || isWEBP(snippet) || isSVG(lowerText) {
			return true
		}
		if u, err := url.Parse(target); err == nil {
			p := strings.ToLower(strings.TrimSpace(u.Path))
			if strings.HasSuffix(p, ".svg") {
				return isSVG(lowerText)
			}
			if strings.HasSuffix(p, ".ico") {
				return isICO(snippet)
			}
			if strings.HasSuffix(p, ".png") {
				return isPNG(snippet)
			}
		}
		return false
	}

	return false
}

func hostnameFromAbsoluteURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	return strings.TrimSpace(u.Hostname())
}

func originFromAbsoluteURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	return strings.TrimSpace(u.Scheme) + "://" + strings.TrimSpace(u.Host)
}

func resolveFaviconIcon(lanUrl string, wanUrl string) string {
	origins := []string{originFromAbsoluteURL(lanUrl), originFromAbsoluteURL(wanUrl)}
	webHosts := make([]string, 0, 2)
	for _, origin := range origins {
		if origin == "" {
			continue
		}
		if !probeWeb(origin) {
			continue
		}
		if icon := resolveFaviconFromOrigin(origin); icon != "" {
			return icon
		}
		if host := hostnameFromAbsoluteURL(origin); host != "" {
			webHosts = append(webHosts, host)
		}
	}
	for _, host := range webHosts {
		candidate := "https://icons.duckduckgo.com/ip3/" + host + ".ico"
		if probeImage(candidate) {
			return candidate
		}
	}
	return ""
}

var faviconLinkRe = regexp.MustCompile(`(?is)<link[^>]+rel=["'](?:shortcut\s+icon|icon|apple-touch-icon|apple-touch-icon-precomposed)["'][^>]*href=["']([^"']+)["']`)

func resolveFaviconFromOrigin(origin string) string {
	origin = strings.TrimRight(strings.TrimSpace(origin), "/")
	if origin == "" {
		return ""
	}
	for _, p := range []string{"/favicon.ico", "/favicon.png", "/favicon.svg"} {
		candidate := origin + p
		if probeImage(candidate) {
			return candidate
		}
	}

	htmlURL := origin + "/"
	client := &http.Client{
		Timeout: 2 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	req, err := http.NewRequest(http.MethodGet, htmlURL, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", "tradis-discovery/1.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml;q=0.9,*/*;q=0.8")
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		io.CopyN(io.Discard, resp.Body, 256)
		return ""
	}
	ct := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Type")))
	if !strings.Contains(ct, "text/html") {
		io.CopyN(io.Discard, resp.Body, 256)
		return ""
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	m := faviconLinkRe.FindSubmatch(body)
	if len(m) < 2 {
		return ""
	}
	href := strings.TrimSpace(string(m[1]))
	if href == "" {
		return ""
	}
	base, err := url.Parse(htmlURL)
	if err != nil {
		return ""
	}
	ref, err := url.Parse(href)
	if err != nil {
		return ""
	}
	candidate := base.ResolveReference(ref).String()
	if probeImage(candidate) {
		return candidate
	}
	return ""
}

func RunNavigationAIBackfill(limit int) int {
	return RunNavigationAIEnrich(limit, false)
}

func RunNavigationAIEnrichByID(navID int, force bool) int {
	s, err := settings.GetSettings()
	if err != nil {
		return 0
	}
	if !s.AiEnabled || navID <= 0 {
		return 0
	}

	navAIEnrichBatchMu.Lock()
	navAIEnrichBatchRunning.Store(true)
	defer func() {
		navAIEnrichBatchRunning.Store(false)
		navAIEnrichBatchMu.Unlock()
	}()

	CleanupOrphanAutoNavigationNow()

	db := database.GetDB()
	var isDeleted int
	var aiGenerated int
	var title sql.NullString
	var category sql.NullString
	var icon sql.NullString
	if err := db.QueryRow("SELECT title, category, icon, is_deleted, ai_generated FROM navigation_items WHERE id = ?", navID).
		Scan(&title, &category, &icon, &isDeleted, &aiGenerated); err != nil {
		return 0
	}
	if isDeleted == 1 {
		return 0
	}

	if !force {
		if aiGenerated == 1 {
			return 0
		}
		needFill := strings.TrimSpace(title.String) == "" ||
			strings.TrimSpace(category.String) == "" || strings.TrimSpace(category.String) == "默认" || strings.TrimSpace(category.String) == "未分类" ||
			strings.TrimSpace(icon.String) == "" || strings.TrimSpace(icon.String) == "mdi-docker"
		if !needFill {
			return 0
		}
	}

	aiEnrichNavigationItem(navID, nil, "", s, force)
	return 1
}

func RunNavigationAIEnrichByTitle(title string, limit int, force bool) int {
	s, err := settings.GetSettings()
	if err != nil {
		return 0
	}
	if !s.AiEnabled {
		return 0
	}
	navAIEnrichBatchMu.Lock()
	navAIEnrichBatchRunning.Store(true)
	defer func() {
		navAIEnrichBatchRunning.Store(false)
		navAIEnrichBatchMu.Unlock()
	}()
	CleanupOrphanAutoNavigationNow()
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
	navAIEnrichBatchMu.Lock()
	navAIEnrichBatchRunning.Store(true)
	defer func() {
		navAIEnrichBatchRunning.Store(false)
		navAIEnrichBatchMu.Unlock()
	}()
	CleanupOrphanAutoNavigationNow()
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
	withNavAIEnrichLock(navID, func() {
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
			strings.TrimSpace(category.String) == "" || strings.TrimSpace(category.String) == "默认" || strings.TrimSpace(category.String) == "未分类" ||
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

	categoryCandidates := make([]string, 0, 12)
	{
		rows, err := db.Query("SELECT category, COUNT(*) AS c FROM navigation_items WHERE is_deleted = 0 AND trim(category) != '' AND category != '默认' AND category != '未分类' GROUP BY category ORDER BY c DESC LIMIT 12")
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var cat string
				var cnt int
				if err := rows.Scan(&cat, &cnt); err != nil {
					continue
				}
				cat = strings.TrimSpace(cat)
				if cat == "" {
					continue
				}
				categoryCandidates = append(categoryCandidates, cat)
			}
		}
	}
	if len(categoryCandidates) == 0 {
		categoryCandidates = []string{"工具", "生产力", "开发", "数据库", "存储", "网络", "监控", "安全", "自动化", "AI工具", "多媒体", "未分类"}
	}

	faviconIcon := resolveFaviconIcon(strings.TrimSpace(lanUrl.String), strings.TrimSpace(wanUrl.String))

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
		"categoryCandidates": categoryCandidates,
		"faviconIcon": faviconIcon,
		"isAuto":      isAuto == 1,
		"force":       force,
	}
	userBytes, _ := json.Marshal(userContent)

	systemPrompt := strings.TrimSpace(s.AiPrompt)
	if systemPrompt == "" {
		systemPrompt = "你是一个导航整理助手。你必须只输出严格 JSON：{\"title\":\"\",\"category\":\"\",\"icon\":\"\"}。title 与 category 必须非空。category 必须尽量给出具体中文分类；仅当完全无法判断时输出 未分类。icon 必须是 http(s) 图标 URL 或 mdi-docker。不要输出解释、推理过程、Markdown、代码块或额外字段。"
	}
	systemPrompt += "\n补充：user JSON 里可能给了 categoryCandidates（分类候选）与 faviconIcon（可用的图标 URL）。如没有更合适的官方图标，可直接用 faviconIcon 作为 icon。"
	systemPrompt += "\n只输出 JSON，不要包含任何其他文本。"

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
	if !strings.HasPrefix(content, "{") || !strings.HasSuffix(content, "}") {
		appendAiLog("navigation", "error", "ai_enrich_parse_failed", map[string]any{
			"navId":   navID,
			"content": content,
		})
		return
	}

	var raw map[string]any
	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		appendAiLog("navigation", "error", "ai_enrich_parse_failed", map[string]any{
			"navId":   navID,
			"content": content,
		})
		return
	}

	for k := range raw {
		if k != "title" && k != "category" && k != "icon" {
			delete(raw, k)
		}
	}
	titleStr, _ := raw["title"].(string)
	categoryStr, _ := raw["category"].(string)
	iconStr, _ := raw["icon"].(string)

	outTitle := normalizeDuplicatePairTitle(strings.TrimSpace(titleStr))
	outCategory := strings.TrimSpace(categoryStr)
	if strings.EqualFold(outCategory, "default") || outCategory == "默认" {
		outCategory = "未分类"
	}
	outIcon := normalizeAIIconValue(strings.TrimSpace(iconStr))
	outIcon = strings.Trim(outIcon, "`")
	if strings.HasPrefix(outIcon, "/icons/ray") {
		outIcon = ""
	}
	if outTitle == "" || outCategory == "" {
		appendAiLog("navigation", "error", "ai_enrich_empty_result", map[string]any{"navId": navID})
		return
	}
	if outIcon == "" {
		outIcon = "mdi-docker"
	}
	if !isAllowedNavigationIconValue(outIcon) {
		outIcon = "mdi-docker"
	}
	outIcon = normalizeAndResolveNavigationIcon(outIcon, strings.TrimSpace(lanUrl.String), strings.TrimSpace(wanUrl.String))

	updateSQL := ""
	if force {
		updateSQL = "UPDATE navigation_items SET title = COALESCE(NULLIF(?, ''), title), category = COALESCE(NULLIF(?, ''), category), icon = COALESCE(NULLIF(?, ''), icon), ai_generated = 1, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND is_deleted = 0"
	} else {
		updateSQL = "UPDATE navigation_items SET title = CASE WHEN trim(title) = '' THEN COALESCE(NULLIF(?, ''), title) ELSE title END, category = CASE WHEN trim(category) = '' OR category = '默认' OR category = '未分类' THEN COALESCE(NULLIF(?, ''), category) ELSE category END, icon = CASE WHEN trim(icon) = '' OR icon = 'mdi-docker' THEN COALESCE(NULLIF(?, ''), icon) ELSE icon END, ai_generated = 1, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND is_deleted = 0 AND ai_generated = 0"
	}
	result, _ := db.Exec(updateSQL, outTitle, outCategory, outIcon, navID)
	affected := int64(0)
	if result != nil {
		affected, _ = result.RowsAffected()
	}
	appendAiLog("navigation", "info", "ai_enrich_done", map[string]any{
		"navId":     navID,
		"latencyMs": time.Since(start).Milliseconds(),
		"updated":   affected > 0,
		"title":     outTitle,
		"category":  outCategory,
		"icon":      outIcon,
	})
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

func normalizeAndResolveNavigationIcon(icon string, lan string, wan string) string {
	out := strings.TrimSpace(icon)
	out = strings.Trim(out, "`")
	if out == "" {
		out = "mdi-docker"
	}
	if strings.HasPrefix(out, "/icons/ray") {
		out = "mdi-docker"
	}
	if !isAllowedNavigationIconValue(out) {
		out = "mdi-docker"
	}
	if strings.HasPrefix(out, "http://") || strings.HasPrefix(out, "https://") {
		if !probeImage(out) {
			out = "mdi-docker"
		}
	}
	if out == "mdi-docker" {
		if candidate := resolveFaviconIcon(lan, wan); candidate != "" {
			out = candidate
		}
	}
	return out
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
