package api

import (
	"bytes"
	"dockerpanel/backend/pkg/settings"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func RegisterAIRoutes(r *gin.RouterGroup) {
	group := r.Group("/ai")
	{
		group.GET("/logs", listAILogs)
		group.POST("/navigation/enrich", enrichNavigationOnce)
		group.POST("/navigation/enrich-by-title", enrichNavigationByTitle)
		group.POST("/test", testAIConnectivity)
	}
}

type aiTestRequest struct {
	Enabled     *bool    `json:"enabled"`
	BaseUrl     *string  `json:"baseUrl"`
	ApiKey      *string  `json:"apiKey"`
	Model       *string  `json:"model"`
	Temperature *float64 `json:"temperature"`
}

func testAIConnectivity(c *gin.Context) {
	var req aiTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "Invalid request", err)
		return
	}

	s, err := settings.GetSettings()
	if err != nil {
		respondError(c, http.StatusInternalServerError, "Failed to get settings", err)
		return
	}

	enabled := s.AiEnabled
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	if !enabled {
		respondError(c, http.StatusBadRequest, "AI is disabled", errors.New("aiEnabled=false"))
		return
	}

	baseUrl := strings.TrimSpace(s.AiBaseUrl)
	if req.BaseUrl != nil {
		baseUrl = strings.TrimSpace(*req.BaseUrl)
	}
	if baseUrl == "" {
		respondError(c, http.StatusBadRequest, "AI baseUrl is required", errors.New("empty baseUrl"))
		return
	}

	model := strings.TrimSpace(s.AiModel)
	if req.Model != nil {
		model = strings.TrimSpace(*req.Model)
	}
	if model == "" {
		respondError(c, http.StatusBadRequest, "AI model is required", errors.New("empty model"))
		return
	}

	apiKey := ""
	if req.ApiKey != nil {
		apiKey = strings.TrimSpace(*req.ApiKey)
	} else {
		apiKey, _ = settings.GetValue("ai_api_key")
		apiKey = strings.TrimSpace(apiKey)
	}
	if apiKey == "" {
		respondError(c, http.StatusBadRequest, "AI apiKey is required", errors.New("empty apiKey"))
		return
	}

	temp := s.AiTemperature
	if req.Temperature != nil {
		temp = *req.Temperature
	}
	if temp < 0 {
		temp = 0
	}
	if temp > 2 {
		temp = 2
	}

	endpoint, err := buildChatCompletionsEndpoint(baseUrl)
	if err != nil {
		respondError(c, http.StatusBadRequest, "AI baseUrl is invalid", err)
		return
	}
	payload := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": "ping"},
		},
		"temperature": temp,
		"max_tokens":  1,
	}
	body, _ := json.Marshal(payload)

	client := &http.Client{Timeout: 20 * time.Second}
	httpReq, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		respondError(c, http.StatusInternalServerError, "Failed to build request", err)
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	start := time.Now()
	resp, err := client.Do(httpReq)
	latency := time.Since(start)
	if err != nil {
		respondError(c, http.StatusBadGateway, "AI request failed", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := readBodySnippet(resp.Body, 4096)
		detail := "status=" + resp.Status
		if snippet != "" {
			detail += " body=" + snippet
		}
		respondErrorWithDetail(c, http.StatusBadGateway, "AI request failed", detail)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":        true,
		"endpoint":  endpoint,
		"latencyMs": latency.Milliseconds(),
	})
}

func buildChatCompletionsEndpoint(raw string) (string, error) {
	u := strings.TrimSpace(raw)
	u = strings.TrimRight(u, "/")
	if u == "" {
		return "", errors.New("empty baseUrl")
	}
	if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
		return "", errors.New("baseUrl must start with http:// or https://")
	}
	if _, err := url.Parse(u); err != nil {
		return "", err
	}
	return u + "/chat/completions", nil
}

func readBodySnippet(r io.Reader, limit int64) (string, error) {
	if limit <= 0 {
		limit = 4096
	}
	b, err := io.ReadAll(io.LimitReader(r, limit))
	if err != nil {
		return "", err
	}
	s := strings.TrimSpace(string(b))
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	if len(s) > 512 {
		s = s[:512]
	}
	return s, nil
}
