package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type APIProxyResult struct {
	URL         string        `json:"url"`
	Alive       bool          `json:"alive"`
	Latency     time.Duration `json:"latency_ms"`
	Models      []string      `json:"models,omitempty"`
	ModelsCount int           `json:"models_count"`
	Unlimited   bool          `json:"unlimited"`
	RateLimit   int           `json:"rate_limit_rpm"`
	Error       string        `json:"error,omitempty"`
	LastChecked time.Time     `json:"last_checked"`
}

type ModelsResponse struct {
	Object string       `json:"object"`
	Data   []ModelEntry `json:"data"`
}
type ModelEntry struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

func CheckAPIProxy(ctx context.Context, baseURL string, timeout time.Duration, checkModels bool) *APIProxyResult {
	result := &APIProxyResult{URL: baseURL, LastChecked: time.Now()}
	baseURL = strings.TrimRight(baseURL, "/")
	client := &http.Client{Timeout: timeout, Transport: &http.Transport{DisableKeepAlives: true}}

	start := time.Now()
	req, _ := http.NewRequestWithContext(ctx, "GET", baseURL+"/v1/models", nil)
	req.Header.Set("User-Agent", "APOXY/1.0")
	resp, err := client.Do(req)
	if err != nil { result.Error = fmt.Sprintf("connection failed: %v", err); return result }
	defer resp.Body.Close()
	result.Latency = time.Since(start)

	if resp.StatusCode == 200 && checkModels {
		var mr ModelsResponse
		if json.NewDecoder(resp.Body).Decode(&mr) == nil {
			result.Alive = true
			result.ModelsCount = len(mr.Data)
			for _, m := range mr.Data { result.Models = append(result.Models, m.ID) }
		}
	} else if resp.StatusCode == 200 || resp.StatusCode == 401 || resp.StatusCode == 403 {
		result.Alive = true
		if resp.StatusCode != 200 { result.Error = fmt.Sprintf("auth required (HTTP %d)", resp.StatusCode) }
	} else { result.Error = fmt.Sprintf("HTTP %d", resp.StatusCode); return result }

	// UNLIMITED CHECK: send with invalid key, 401/403 = open endpoint
	result.Unlimited = checkUnlimited(ctx, baseURL, client)

	// RATE LIMIT DETECTION: burst 10 requests, count 429s
	if result.Alive { result.RateLimit = detectRateLimit(ctx, baseURL, client) }
	if result.Alive && result.ModelsCount > 0 { testChatCompletion(ctx, baseURL, client, result) }
	return result
}

func checkUnlimited(ctx context.Context, baseURL string, client *http.Client) bool {
	req, _ := http.NewRequestWithContext(ctx, "POST", baseURL+"/v1/chat/completions",
		strings.NewReader(`{"model":"gpt-3.5-turbo","messages":[{"role":"user","content":"hi"}],"max_tokens":1}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer INVALID_KEY_APOXY_UNLIMITED_CHECK")
	req.Header.Set("User-Agent", "APOXY/1.0")
	resp, err := client.Do(req)
	if err != nil { return false }
	defer resp.Body.Close()
	return resp.StatusCode == 401 || resp.StatusCode == 403
}

func detectRateLimit(ctx context.Context, baseURL string, client *http.Client) int {
	hits := 0
	for i := 0; i < 10; i++ {
		req, _ := http.NewRequestWithContext(ctx, "GET", baseURL+"/v1/models", nil)
		req.Header.Set("User-Agent", "APOXY/1.0")
		resp, _ := client.Do(req)
		if resp != nil { 
			if resp.StatusCode == 429 { hits++ }
			resp.Body.Close() 
		}
	}
	if hits > 0 { return hits }
	return 999
}

func testChatCompletion(ctx context.Context, baseURL string, client *http.Client, result *APIProxyResult) bool {
	chatURL := baseURL + "/v1/chat/completions"
	testModel := "gpt-3.5-turbo"
	for _, m := range result.Models {
		if strings.Contains(strings.ToLower(m), "flash") || strings.Contains(strings.ToLower(m), "mini") || strings.Contains(strings.ToLower(m), "free") {
			testModel = m; break
		}
	}
	if len(result.Models) > 0 && testModel == "gpt-3.5-turbo" { testModel = result.Models[0] }
	body := fmt.Sprintf(`{"model":"%s","messages":[{"role":"user","content":"hi"}],"max_tokens":5}`, testModel)
	req, _ := http.NewRequestWithContext(ctx, "POST", chatURL, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "APOXY/1.0")
	resp, err := client.Do(req)
	if err != nil { return false }
	defer resp.Body.Close()
	return resp.StatusCode == 200 || resp.StatusCode == 401 || resp.StatusCode == 403
}
