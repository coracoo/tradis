package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var gitSyncMu sync.Mutex

type gitSyncIndexItem struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Logo        string `json:"logo"`
	Website     string `json:"website"`
	File        string `json:"file"`
}

type gitSyncIndexFile struct {
	Note          string             `json:"note,omitempty"`
	TemplatesNote string             `json:"templates_note,omitempty"`
	GeneratedAt   string             `json:"generated_at,omitempty"`
	Templates     []gitSyncIndexItem `json:"templates"`
}

type gitSyncResult struct {
	UpdatedCount  int    `json:"updated_count"`
	TotalCount    int    `json:"total_count"`
	Note          string `json:"note"`
	TemplatesNote string `json:"templates_note"`
}

type exportTemplateFile struct {
	ID          uint      `json:"id"`
	Name        string    `json:"name"`
	Category    string    `json:"category"`
	Description string    `json:"description"`
	Version     string    `json:"version"`
	Logo        string    `json:"logo"`
	Website     string    `json:"website"`
	Tutorial    string    `json:"tutorial"`
	Dotenv      string    `json:"dotenv"`
	Compose     string    `json:"compose"`
	Screenshots []string  `json:"screenshots"`
	Schema      Variables `json:"schema"`
	Enabled     bool      `json:"enabled"`
}

// SyncTemplatesToGitSync 同步模板数据到 data/git_sync，并将变更推送到 GitHub
func SyncTemplatesToGitSync(db *gorm.DB) error {
	_, err := syncTemplatesToGitSync(db, true)
	return err
}

// SyncTemplatesToGithubHandler 手动触发同步到 GitHub
func SyncTemplatesToGithubHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		res, err := syncTemplatesToGitSync(db, true)
		if err != nil {
			c.JSON(500, gin.H{"error": "同步失败", "detail": err.Error()})
			return
		}
		c.JSON(200, gin.H{
			"message":        "同步完成",
			"updated_count":  res.UpdatedCount,
			"total_count":    res.TotalCount,
			"note":           res.Note,
			"templates_note": res.TemplatesNote,
		})
	}
}

// syncTemplatesToGitSync 执行导出与推送（串行化执行，避免并发写文件和 git 冲突）
func syncTemplatesToGitSync(db *gorm.DB, doPush bool) (*gitSyncResult, error) {
	gitSyncMu.Lock()
	defer gitSyncMu.Unlock()

	var templates []Template
	if err := db.Find(&templates).Error; err != nil {
		return nil, fmt.Errorf("查询模板数据失败: %w", err)
	}

	baseDir := "data/git_sync"
	tplDir := filepath.Join(baseDir, "templates")
	if err := os.MkdirAll(tplDir, 0755); err != nil {
		return nil, fmt.Errorf("创建 git_sync 目录失败: %w", err)
	}

	oldEnabled, err := loadEnabledFromIndex(filepath.Join(baseDir, "index.json"))
	if err != nil {
		oldEnabled = map[string]struct{}{}
	}

	newEnabled := make(map[string]exportTemplateFile)
	for _, t := range templates {
		if !t.Enabled {
			continue
		}
		tt := t
		normalizeTemplateDotenvBySchema(&tt)
		name := strings.TrimSpace(t.Name)
		if name == "" {
			name = fmt.Sprintf("template_%d", t.ID)
		}
		newEnabled[name] = exportTemplateFile{
			ID:          tt.ID,
			Name:        name,
			Category:    tt.Category,
			Description: tt.Description,
			Version:     tt.Version,
			Logo:        tt.Logo,
			Website:     tt.Website,
			Tutorial:    tt.Tutorial,
			Dotenv:      tt.Dotenv,
			Compose:     tt.Compose,
			Screenshots: []string(tt.Screenshots),
			Schema:      Variables(tt.Schema),
			Enabled:     tt.Enabled,
		}
	}

	updatedCount := 0

	newNames := make([]string, 0, len(newEnabled))
	for name := range newEnabled {
		newNames = append(newNames, name)
	}
	sort.Strings(newNames)

	indexItems := make([]gitSyncIndexItem, 0, len(newNames))
	for _, name := range newNames {
		exp := newEnabled[name]
		filename := filepath.Join(tplDir, name+".json")
		newBytes, err := json.MarshalIndent(exp, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("序列化模板失败(%s): %w", name, err)
		}
		newBytes = append(newBytes, '\n')

		oldBytes, readErr := os.ReadFile(filename)
		if _, ok := oldEnabled[name]; !ok {
			updatedCount++
		} else if readErr == nil {
			if !bytes.Equal(oldBytes, newBytes) {
				updatedCount++
			}
		} else if errors.Is(readErr, fs.ErrNotExist) {
			updatedCount++
		}

		if err := os.WriteFile(filename, newBytes, 0644); err != nil {
			return nil, fmt.Errorf("写入模板文件失败(%s): %w", name, err)
		}

		indexItems = append(indexItems, gitSyncIndexItem{
			ID:          exp.ID,
			Name:        exp.Name,
			Category:    exp.Category,
			Description: exp.Description,
			Version:     exp.Version,
			Logo:        exp.Logo,
			Website:     exp.Website,
			File:        fmt.Sprintf("templates/%s.json", name),
		})
	}

	removedCount := 0
	entries, _ := os.ReadDir(tplDir)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		fn := e.Name()
		if !strings.HasSuffix(strings.ToLower(fn), ".json") {
			continue
		}
		name := strings.TrimSuffix(fn, filepath.Ext(fn))
		if _, ok := newEnabled[name]; ok {
			continue
		}
		_ = os.Remove(filepath.Join(tplDir, fn))
		if _, ok := oldEnabled[name]; ok {
			removedCount++
		}
	}
	updatedCount += removedCount

	totalCount := len(newEnabled)
	note := fmt.Sprintf("%s，更新项目%d个", time.Now().Format("2006-01-02 15:04:05"), updatedCount)
	templatesNote := fmt.Sprintf("累计项目%d个", totalCount)

	indexFile := filepath.Join(baseDir, "index.json")
	idx := gitSyncIndexFile{
		Note:          note,
		TemplatesNote: templatesNote,
		GeneratedAt:   time.Now().Format(time.RFC3339),
		Templates:     indexItems,
	}
	idxBytes, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("序列化 index.json 失败: %w", err)
	}
	idxBytes = append(idxBytes, '\n')
	if err := os.WriteFile(indexFile, idxBytes, 0644); err != nil {
		return nil, fmt.Errorf("写入 index.json 失败: %w", err)
	}

	if doPush {
		auto := strings.TrimSpace(os.Getenv("GIT_SYNC_AUTO_PUSH"))
		if auto == "" {
			auto = "1"
		}
		if auto == "1" {
			if err := runGitSync(baseDir, note+"\n"+templatesNote); err != nil {
				return nil, err
			}
		}
	}

	return &gitSyncResult{UpdatedCount: updatedCount, TotalCount: totalCount, Note: note, TemplatesNote: templatesNote}, nil
}

// loadEnabledFromIndex 从 index.json 读取启用模板的 name 列表
func loadEnabledFromIndex(indexPath string) (map[string]struct{}, error) {
	data, err := os.ReadFile(indexPath)
	if err != nil {
		return nil, err
	}
	var idx gitSyncIndexFile
	if err := json.Unmarshal(data, &idx); err == nil && len(idx.Templates) > 0 {
		out := make(map[string]struct{}, len(idx.Templates))
		for _, it := range idx.Templates {
			n := strings.TrimSpace(it.Name)
			if n != "" {
				out[n] = struct{}{}
			}
		}
		return out, nil
	}

	var legacy struct {
		Templates []gitSyncIndexItem `json:"templates"`
	}
	if err := json.Unmarshal(data, &legacy); err != nil {
		return nil, err
	}
	out := make(map[string]struct{}, len(legacy.Templates))
	for _, it := range legacy.Templates {
		n := strings.TrimSpace(it.Name)
		if n != "" {
			out[n] = struct{}{}
		}
	}
	return out, nil
}

// runGitSync 调用 git_sync 目录下的脚本或直接执行 git 命令
func runGitSync(baseDir string, commitMessage string) error {
	absBaseDir, err := filepath.Abs(baseDir)
	if err != nil {
		absBaseDir = baseDir
	}

	scriptPath := filepath.Join(absBaseDir, "cf-tem-push.sh")
	repoURL := defaultString(os.Getenv("GIT_SYNC_REPO_URL"), "git@github.com:coracoo/tradis_templates.git")
	if fi, err := os.Stat(scriptPath); err == nil && !fi.IsDir() {
		var outBuf bytes.Buffer
		cmd := exec.Command("bash", scriptPath)
		cmd.Dir = absBaseDir
		cmd.Stdout = io.MultiWriter(os.Stdout, &outBuf)
		cmd.Stderr = io.MultiWriter(os.Stderr, &outBuf)
		cmd.Env = append(os.Environ(),
			"GIT_SYNC_COMMIT_MESSAGE="+commitMessage,
			"GIT_SYNC_REPO_URL="+repoURL,
		)
		if err := cmd.Run(); err != nil {
			if fbErr := runGitSyncFallback(baseDir, commitMessage, repoURL); fbErr == nil {
				return nil
			}
			out := strings.TrimSpace(outBuf.String())
			if out == "" {
				return fmt.Errorf("git 推送失败: %w", err)
			}
			if len(out) > 2000 {
				out = out[:2000]
			}
			return fmt.Errorf("git 推送失败: %w; 输出: %s", err, out)
		}
		return nil
	}

	return runGitSyncFallback(absBaseDir, commitMessage, repoURL)
}

// runGitSyncFallback 当脚本不可用/失败时，使用 git 命令直接推送
func runGitSyncFallback(baseDir string, commitMessage string, repoURL string) error {
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = baseDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}

	msg := strings.TrimSpace(commitMessage)
	if msg == "" {
		msg = fmt.Sprintf("%s，更新项目0个", time.Now().Format("2006-01-02 15:04:05"))
	}
	lines := strings.Split(msg, "\n")
	args := []string{"commit"}
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		args = append(args, "-m", l)
	}
	cmd = exec.Command("git", args...)
	cmd.Dir = baseDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()

	if strings.TrimSpace(repoURL) != "" {
		cmd = exec.Command("git", "remote", "get-url", "origin")
		cmd.Dir = baseDir
		var buf bytes.Buffer
		cmd.Stdout = &buf
		cmd.Stderr = &buf
		if err := cmd.Run(); err != nil {
			cmd = exec.Command("git", "remote", "add", "origin", repoURL)
			cmd.Dir = baseDir
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			_ = cmd.Run()
		}
	}

	cmd = exec.Command("git", "push")
	cmd.Dir = baseDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// defaultString 当 v 为空时返回 d
func defaultString(v, d string) string {
	if strings.TrimSpace(v) == "" {
		return d
	}
	return v
}
