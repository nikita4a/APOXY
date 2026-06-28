package proxy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

var knownAPIPatterns = []string{
	` + "`(?i)https?://[^\\s<>\"]+/v1\\b`" + `,
	` + "`(?i)https?://[^\\s<>\"]+\\.trycloudflare\\.com/v1`" + `,
	` + "`(?i)https?://[^\\s<>\"]+\\.ngrok\\.\\w+/v1`" + `,
	` + "`(?i)https?://[^\\s<>\"]+\\.workers\\.dev/v1`" + `,
	` + "`(?i)https?://[^\\s<>\"]+\\.vercel\\.app/v1`" + `,
	` + "`(?i)https?://[^\\s<>\"]+\\.railway\\.app/v1`" + `,
	` + "`(?i)https?://[^\\s<>\"]+\\.fly\\.dev/v1`" + `,
}

type RawAPIEndpoint struct{ URL, Source string }

func ScrapeAPISources(ctx context.Context, sources []string) ([]RawAPIEndpoint, error) {
	var results []RawAPIEndpoint
	seen := make(map[string]bool)
	for _, src := range sources {
		select {
		case <-ctx.Done(): return results, ctx.Err()
		default:
		}
		endpoints, err := scrapeSingleSource(ctx, src)
		if err != nil { continue }
		for _, ep := range endpoints {
			n := normalizeURL(ep.URL)
			if n == "" || seen[n] { continue }
			seen[n] = true; ep.URL = n
			results = append(results, ep)
		}
	}
	return results, nil
}

func scrapeSingleSource(ctx context.Context, url string) ([]RawAPIEndpoint, error) {
	sctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(sctx, "GET", url, nil)
	req.Header.Set("User-Agent", "APOXY/1.0")
	resp, err := http.DefaultClient.Do(req)
	if err != nil { return nil, err }
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK { return nil, fmt.Errorf("HTTP %d", resp.StatusCode) }
	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	bodyStr := string(bodyBytes)
	var results []RawAPIEndpoint
	for _, pattern := range knownAPIPatterns {
		re := regexp.MustCompile(pattern)
		for _, match := range re.FindAllString(bodyStr, -1) {
			match = strings.TrimSpace(match); match = strings.TrimRight(match, "/")
			results = append(results, RawAPIEndpoint{URL: match, Source: url})
		}
	}
	re := regexp.MustCompile(` + "`(?i)https?://[^\\s<>\"')]+\\b/v1\\b[^\\s<>\"')]*`" + `)
	for _, match := range re.FindAllString(bodyStr, -1) {
		match = strings.TrimSpace(match); match = strings.TrimRight(match, "/")
		results = append(results, RawAPIEndpoint{URL: match, Source: url})
	}
	return results, nil
}

func normalizeURL(raw string) string {
	raw = strings.TrimSpace(raw); raw = strings.TrimRight(raw, "/")
	if !strings.HasPrefix(raw, "http") { return "" }
	return strings.ReplaceAll(raw, " ", "")
}

func AddBuiltinSources() []string { return []string{} }
