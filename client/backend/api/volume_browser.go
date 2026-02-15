package api

import (
	"context"
	"crypto/rand"
	"dockerpanel/backend/pkg/database"
	"dockerpanel/backend/pkg/docker"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/go-connections/nat"
	"github.com/gin-gonic/gin"
)

const (
	volumeBrowseImage = "filebrowser/filebrowser:latest"
	volumeBrowseTTL   = 2 * time.Minute
)

type volumeBrowseSession struct {
	ID          string
	VolumeName  string
	ContainerID string
	TargetHost  string
	ReadOnly    bool
	LastSeen    time.Time
}

var (
	volumeBrowseMu       sync.Mutex
	volumeBrowseSessions = map[string]*volumeBrowseSession{}
	volumeBrowseOnce     sync.Once
)

func startVolumeBrowse(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	volumeName := strings.TrimSpace(c.Param("name"))
	if volumeName == "" {
		respondError(c, http.StatusBadRequest, "invalid volume", nil)
		return
	}

	cli, err := docker.NewDockerClient()
	if err != nil {
		respondError(c, http.StatusInternalServerError, "docker client init failed", err)
		return
	}
	defer cli.Close()

	if _, err := cli.VolumeInspect(context.Background(), volumeName); err != nil {
		respondError(c, http.StatusNotFound, "volume not found", err)
		return
	}

	readOnly := isVolumeInUse(context.Background(), cli, volumeName)

	if err := ensureImage(context.Background(), cli, volumeBrowseImage); err != nil {
		respondError(c, http.StatusInternalServerError, "pull image failed", err)
		return
	}

	sid := newSessionID()
	containerName := "tradis-volume-browser-" + sid

	baseURL := "/api/volumes/browse/" + sid + "/fb"
	env := []string{
		"FB_NOAUTH=true",
		"FB_BASEURL=" + baseURL,
	}

	exposed := nat.PortSet{
		nat.Port("80/tcp"): struct{}{},
	}

	cfg := &container.Config{
		Image:        volumeBrowseImage,
		Env:          env,
		ExposedPorts: exposed,
		Labels: map[string]string{
			"tradis.managed": "true",
			"tradis.role":    "volume-browser",
			"tradis.session": sid,
			"tradis.volume":  volumeName,
		},
	}
	hostCfg := &container.HostConfig{
		PortBindings: nat.PortMap{
			nat.Port("80/tcp"): []nat.PortBinding{{HostIP: "127.0.0.1", HostPort: "0"}},
		},
		Mounts: []mount.Mount{
			{
				Type:     mount.TypeVolume,
				Source:   volumeName,
				Target:   "/srv",
				ReadOnly: readOnly,
			},
		},
	}

	createResp, err := cli.ContainerCreate(context.Background(), cfg, hostCfg, nil, nil, containerName)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "create helper container failed", err)
		return
	}
	if err := cli.ContainerStart(context.Background(), createResp.ID, types.ContainerStartOptions{}); err != nil {
		_ = cli.ContainerRemove(context.Background(), createResp.ID, types.ContainerRemoveOptions{Force: true})
		respondError(c, http.StatusInternalServerError, "start helper container failed", err)
		return
	}

	targetHost := ""
	for i := 0; i < 15; i++ {
		ins, err := cli.ContainerInspect(context.Background(), createResp.ID)
		if err == nil {
			targetHost = pickContainerHost(ins)
		}
		if targetHost != "" {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if targetHost == "" {
		_ = cli.ContainerRemove(context.Background(), createResp.ID, types.ContainerRemoveOptions{Force: true, RemoveVolumes: true})
		respondError(c, http.StatusInternalServerError, "resolve helper container address failed", nil)
		return
	}

	volumeBrowseMu.Lock()
	volumeBrowseSessions[sid] = &volumeBrowseSession{
		ID:          sid,
		VolumeName:  volumeName,
		ContainerID: createResp.ID,
		TargetHost:  targetHost,
		ReadOnly:    readOnly,
		LastSeen:    time.Now(),
	}
	volumeBrowseMu.Unlock()

	volumeBrowseOnce.Do(startVolumeBrowseReaper)

	_ = database.SaveNotification(&database.Notification{
		Type:    "info",
		Message: fmt.Sprintf("卷文件浏览已启动：%s", volumeName),
		Read:    false,
	})

	c.JSON(http.StatusOK, gin.H{
		"sessionId": sid,
		"url":       "/api/volumes/browse/" + sid + "/ui",
		"readOnly":  readOnly,
	})
}

func volumeBrowseUI(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	sid := strings.TrimSpace(c.Param("sid"))
	s := getVolumeBrowseSession(sid)
	if s == nil {
		respondError(c, http.StatusNotFound, "session not found", nil)
		return
	}
	token := strings.TrimSpace(c.Query("token"))
	if token != "" {
		c.SetCookie("token", token, 86400, "/", "", false, true)
	}
	tp := "/api/volumes/browse/" + sid + "/fb/"
	hb := "/api/volumes/browse/" + sid + "/heartbeat"
	cl := "/api/volumes/browse/" + sid + "/close"

	html := `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>卷文件浏览器</title>
  <style>
    html, body { height: 100%; margin: 0; background: #0b0f19; }
    .bar { height: 44px; display:flex; align-items:center; padding:0 12px; color:#e5e7eb; font: 14px system-ui, -apple-system, Segoe UI, Roboto, sans-serif; }
    .bar .hint { opacity: 0.75; }
    iframe { width: 100%; height: calc(100% - 44px); border: 0; background: #fff; }
  </style>
</head>
<body>
  <div class="bar">
    <div class="hint">关闭此页面将自动清理临时容器。</div>
  </div>
  <iframe id="fb"></iframe>
  <script>
    const qs = new URLSearchParams(location.search);
    const token = qs.get('token') || '';
    const iframe = document.getElementById('fb');
    const iframeUrl = new URL('` + tp + `', location.origin);
    if (token) iframeUrl.searchParams.set('token', token);
    iframe.src = iframeUrl.toString();

    const hbUrl = new URL('` + hb + `', location.origin);
    const closeUrl = new URL('` + cl + `', location.origin);
    if (token) { hbUrl.searchParams.set('token', token); closeUrl.searchParams.set('token', token); }

    const tick = () => fetch(hbUrl.toString(), { method: 'POST', keepalive: true }).catch(() => {});
    tick();
    const timer = setInterval(tick, 15000);

    const close = () => {
      clearInterval(timer);
      try {
        if (navigator.sendBeacon) {
          navigator.sendBeacon(closeUrl.toString(), '');
        } else {
          fetch(closeUrl.toString(), { method: 'POST', keepalive: true }).catch(() => {});
        }
      } catch (e) {}
    };
    window.addEventListener('beforeunload', close);
  </script>
</body>
</html>`

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, html)
}

func volumeBrowseHeartbeat(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}
	sid := strings.TrimSpace(c.Param("sid"))
	volumeBrowseMu.Lock()
	if s, ok := volumeBrowseSessions[sid]; ok && s != nil {
		s.LastSeen = time.Now()
	}
	volumeBrowseMu.Unlock()
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func volumeBrowseClose(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}
	sid := strings.TrimSpace(c.Param("sid"))
	_ = closeVolumeBrowseSession(sid)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func volumeBrowseProxy(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	sid := strings.TrimSpace(c.Param("sid"))
	s := getVolumeBrowseSession(sid)
	if s == nil {
		respondError(c, http.StatusNotFound, "session not found", nil)
		return
	}
	if token := strings.TrimSpace(c.Query("token")); token != "" {
		c.SetCookie("token", token, 86400, "/", "", false, true)
	}

	volumeBrowseMu.Lock()
	s.LastSeen = time.Now()
	volumeBrowseMu.Unlock()

	subPath := c.Param("path")
	if strings.TrimSpace(subPath) == "" {
		subPath = "/"
	}
	basePrefix := "/api/volumes/browse/" + sid + "/fb"

	target := &url.URL{Scheme: "http", Host: s.TargetHost}
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Director = func(req *http.Request) {
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.URL.Path = subPath
		req.URL.RawPath = subPath
		q := req.URL.Query()
		q.Del("token")
		req.URL.RawQuery = q.Encode()
		req.Host = target.Host
		req.Header.Del("X-Forwarded-Host")
		req.Header.Del("X-Forwarded-Proto")
		req.Header.Del("X-Forwarded-For")
	}
	proxy.ModifyResponse = func(resp *http.Response) error {
		ct := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Type")))
		isHTML := strings.Contains(ct, "text/html")
		isJS := strings.Contains(ct, "application/javascript") || strings.Contains(ct, "text/javascript") || strings.Contains(ct, "application/x-javascript")
		if !isHTML && !isJS {
			return nil
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
		if err != nil {
			return err
		}
		_ = resp.Body.Close()

		s := string(body)
		if isHTML {
			s = strings.ReplaceAll(s, "\"/static/", "\""+basePrefix+"/static/")
			s = strings.ReplaceAll(s, "'/static/", "'"+basePrefix+"/static/")
			s = strings.ReplaceAll(s, "src=/static/", "src="+basePrefix+"/static/")
			s = strings.ReplaceAll(s, "href=/static/", "href="+basePrefix+"/static/")
			s = strings.ReplaceAll(s, "\"/favicon", "\""+basePrefix+"/favicon")
			s = strings.ReplaceAll(s, "'/favicon", "'"+basePrefix+"/favicon")
		}
		if isHTML || isJS {
			s = strings.ReplaceAll(s, "\"/api/", "\""+basePrefix+"/api/")
			s = strings.ReplaceAll(s, "'/api/", "'"+basePrefix+"/api/")
		}

		resp.Body = io.NopCloser(strings.NewReader(s))
		resp.ContentLength = int64(len(s))
		resp.Header.Del("Content-Length")
		return nil
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		respondError(c, http.StatusBadGateway, "proxy failed", err)
	}
	proxy.ServeHTTP(c.Writer, c.Request)
}

func startVolumeBrowseReaper() {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			expired := make([]string, 0, 8)
			expiredSet := map[string]struct{}{}
			now := time.Now()
			snapshot := map[string]*volumeBrowseSession{}
			volumeBrowseMu.Lock()
			for sid, s := range volumeBrowseSessions {
				snapshot[sid] = s
				if s == nil {
					expired = append(expired, sid)
					expiredSet[sid] = struct{}{}
					continue
				}
				if now.Sub(s.LastSeen) > volumeBrowseTTL {
					expired = append(expired, sid)
					expiredSet[sid] = struct{}{}
				}
			}
			volumeBrowseMu.Unlock()
			for _, sid := range expired {
				_ = closeVolumeBrowseSession(sid)
			}
			cleanupOrphanVolumeBrowsers(snapshot, expiredSet)
		}
	}()
}

func cleanupOrphanVolumeBrowsers(sessions map[string]*volumeBrowseSession, expired map[string]struct{}) {
	cli, err := docker.NewDockerClient()
	if err != nil {
		return
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	args := filters.NewArgs(
		filters.Arg("label", "tradis.role=volume-browser"),
	)
	list, err := cli.ContainerList(ctx, types.ContainerListOptions{All: true, Filters: args})
	if err != nil {
		return
	}
	for _, c := range list {
		sid := strings.TrimSpace(c.Labels["tradis.session"])
		if sid == "" {
			_ = cli.ContainerRemove(ctx, c.ID, types.ContainerRemoveOptions{Force: true, RemoveVolumes: true})
			continue
		}
		if _, ok := expired[sid]; ok {
			_ = cli.ContainerRemove(ctx, c.ID, types.ContainerRemoveOptions{Force: true, RemoveVolumes: true})
			continue
		}
		s := sessions[sid]
		if s == nil {
			_ = cli.ContainerRemove(ctx, c.ID, types.ContainerRemoveOptions{Force: true, RemoveVolumes: true})
			continue
		}
		if s.ContainerID != "" && s.ContainerID != c.ID {
			_ = cli.ContainerRemove(ctx, c.ID, types.ContainerRemoveOptions{Force: true, RemoveVolumes: true})
			continue
		}
	}
}

func closeVolumeBrowseSession(sid string) error {
	s := (*volumeBrowseSession)(nil)
	volumeBrowseMu.Lock()
	if v, ok := volumeBrowseSessions[sid]; ok {
		s = v
		delete(volumeBrowseSessions, sid)
	}
	volumeBrowseMu.Unlock()
	if s == nil {
		return nil
	}

	cli, err := docker.NewDockerClient()
	if err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = cli.ContainerRemove(ctx, s.ContainerID, types.ContainerRemoveOptions{Force: true, RemoveVolumes: true})
		cli.Close()
	}

	_ = database.SaveNotification(&database.Notification{
		Type:    "info",
		Message: fmt.Sprintf("卷文件浏览已关闭：%s", s.VolumeName),
		Read:    false,
	})
	return nil
}

func getVolumeBrowseSession(sid string) *volumeBrowseSession {
	volumeBrowseMu.Lock()
	defer volumeBrowseMu.Unlock()
	s := volumeBrowseSessions[sid]
	if s == nil {
		return nil
	}
	cp := *s
	return &cp
}

func newSessionID() string {
	var b [12]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func pickContainerHost(ins types.ContainerJSON) string {
	if ins.NetworkSettings != nil {
		if ins.NetworkSettings.Ports != nil {
			if bindings, ok := ins.NetworkSettings.Ports[nat.Port("80/tcp")]; ok && len(bindings) > 0 {
				hostPort := strings.TrimSpace(bindings[0].HostPort)
				if hostPort != "" {
					return "127.0.0.1:" + hostPort
				}
			}
		}
		if ip := strings.TrimSpace(ins.NetworkSettings.IPAddress); ip != "" {
			return ip + ":80"
		}
		for _, n := range ins.NetworkSettings.Networks {
			if n == nil {
				continue
			}
			if ip := strings.TrimSpace(n.IPAddress); ip != "" {
				return ip + ":80"
			}
		}
	}
	return ""
}

func ensureImage(ctx context.Context, cli *docker.Client, image string) error {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	if _, _, err := cli.ImageInspectWithRaw(ctx, image); err == nil {
		return nil
	}
	rc, err := cli.ImagePull(ctx, image, types.ImagePullOptions{})
	if err != nil {
		return err
	}
	defer rc.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(rc, 2*1024*1024))
	return nil
}

func isVolumeInUse(ctx context.Context, cli *docker.Client, volName string) bool {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	containers, err := cli.ContainerList(ctx, types.ContainerListOptions{All: true})
	if err != nil {
		return true
	}
	for _, ctr := range containers {
		for _, m := range ctr.Mounts {
			if m.Type == "volume" && strings.TrimSpace(m.Name) == volName {
				return true
			}
		}
	}
	return false
}

func requireAdmin(c *gin.Context) bool {
	u := strings.TrimSpace(c.GetString("username"))
	if u == "admin" {
		return true
	}
	respondError(c, http.StatusForbidden, "管理员权限 required", nil)
	return false
}
