package system

import (
	"context"
	"dockerpanel/backend/pkg/database"
	"dockerpanel/backend/pkg/docker"
	"dockerpanel/backend/pkg/settings"
	"io"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
)

const volumeBackupContainerName = "tradis-volume-backup"

func EnsureVolumeBackupContainer(s settings.Settings) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cli, err := docker.NewDockerClient()
	if err != nil {
		saveNotification("error", "卷备份容器启用失败：Docker 客户端初始化失败")
		return
	}
	defer cli.Close()

	if !s.VolumeBackupEnabled {
		_ = removeContainerByName(ctx, cli, volumeBackupContainerName)
		return
	}

	vols := normalizeStringList(s.VolumeBackupVolumes)
	if len(vols) == 0 {
		_ = removeContainerByName(ctx, cli, volumeBackupContainerName)
		saveNotification("warning", "卷备份未启用：未选择需要备份的卷")
		return
	}

	image := strings.TrimSpace(s.VolumeBackupImage)
	if image == "" {
		image = "offen/docker-volume-backup:latest"
	}

	_ = removeContainerByName(ctx, cli, volumeBackupContainerName)

	if err := ensureImageReady(ctx, cli, image); err != nil {
		saveNotification("error", "卷备份镜像拉取失败："+redactDockerError(err))
		return
	}

	env := parseEnvText(s.VolumeBackupEnv)

	mounts := make([]mount.Mount, 0, len(vols)+2)
	for _, v := range vols {
		mounts = append(mounts, mount.Mount{
			Type:     mount.TypeVolume,
			Source:   v,
			Target:   "/backup/" + sanitizeVolumeTarget(v),
			ReadOnly: true,
		})
	}

	if s.VolumeBackupMountDockerSock {
		mounts = append(mounts, mount.Mount{
			Type:     mount.TypeBind,
			Source:   "/var/run/docker.sock",
			Target:   "/var/run/docker.sock",
			ReadOnly: true,
		})
	}

	archiveDir := strings.TrimSpace(s.VolumeBackupArchiveDir)
	if archiveDir != "" {
		archiveDir = filepath.Clean(archiveDir)
		mounts = append(mounts, mount.Mount{
			Type:   mount.TypeBind,
			Source: archiveDir,
			Target: "/archive",
		})
	}

	cfg := &container.Config{
		Image: image,
		Env:   env,
		Labels: map[string]string{
			"tradis.managed": "true",
			"tradis.role":    "volume-backup",
		},
	}
	hostCfg := &container.HostConfig{
		RestartPolicy: container.RestartPolicy{Name: "always"},
		Mounts:        mounts,
	}

	resp, err := cli.ContainerCreate(ctx, cfg, hostCfg, nil, nil, volumeBackupContainerName)
	if err != nil {
		saveNotification("error", "卷备份容器创建失败："+redactDockerError(err))
		return
	}

	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		saveNotification("error", "卷备份容器启动失败："+redactDockerError(err))
		_ = cli.ContainerRemove(ctx, resp.ID, types.ContainerRemoveOptions{Force: true, RemoveVolumes: false})
		return
	}

	saveNotification("success", "卷备份已启用（docker-volume-backup）")
}

func ensureImageReady(ctx context.Context, cli *docker.Client, image string) error {
	image = strings.TrimSpace(image)
	if image == "" {
		return nil
	}
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

func removeContainerByName(ctx context.Context, cli *docker.Client, name string) error {
	id := ""
	args := filters.NewArgs(filters.Arg("name", name))
	list, err := cli.ContainerList(ctx, types.ContainerListOptions{All: true, Filters: args})
	if err != nil {
		return err
	}
	for _, c := range list {
		for _, n := range c.Names {
			if strings.TrimPrefix(n, "/") == name {
				id = c.ID
				break
			}
		}
		if id != "" {
			break
		}
	}
	if id == "" {
		return nil
	}
	_ = cli.ContainerRemove(ctx, id, types.ContainerRemoveOptions{Force: true, RemoveVolumes: false})
	return nil
}

func saveNotification(tp string, msg string) {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return
	}
	if tp == "" {
		tp = "info"
	}
	_ = database.SaveNotification(&database.Notification{Type: tp, Message: msg, Read: false})
}

func redactDockerError(err error) string {
	if err == nil {
		return ""
	}
	s := strings.TrimSpace(err.Error())
	s = settings.RedactAppStoreURL(s)
	if len(s) > 200 {
		s = s[:200]
	}
	return s
}

var envKeyRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func parseEnvText(raw string) []string {
	lines := strings.Split(raw, "\n")
	out := make([]string, 0, len(lines))
	index := make(map[string]int, 32)
	for _, line := range lines {
		s := strings.TrimSpace(line)
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}
		if !strings.Contains(s, "=") {
			continue
		}
		parts := strings.SplitN(s, "=", 2)
		key := strings.TrimSpace(parts[0])
		val := parts[1]
		if key == "" || !envKeyRe.MatchString(key) {
			continue
		}
		item := key + "=" + val
		if idx, ok := index[key]; ok {
			out[idx] = item
			continue
		}
		index[key] = len(out)
		out = append(out, item)
	}
	return out
}

func normalizeStringList(list []string) []string {
	out := make([]string, 0, len(list))
	seen := make(map[string]struct{}, len(list))
	for _, v := range list {
		s := strings.TrimSpace(v)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func sanitizeVolumeTarget(v string) string {
	s := strings.TrimSpace(v)
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, "\\", "_")
	s = strings.ReplaceAll(s, ":", "_")
	s = strings.TrimSpace(s)
	if s == "" {
		return "volume"
	}
	return s
}
