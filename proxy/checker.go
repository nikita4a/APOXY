package proxy

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/proxy"
)

// RawProxy holds scraped, unchecked proxy information.
type RawProxy struct {
	IPPort        string
	SuggestedType string // "http", "socks4", "socks5" or ""
}

// CheckedProxy holds the result of a verified proxy.
type CheckedProxy struct {
	IP          string        `json:"ip"`
	Port        int           `json:"port"`
	Protocol    string        `json:"protocol"` // "http", "socks4", "socks5"
	Ping        time.Duration `json:"ping"`
	CountryCode string        `json:"country_code"`
	CountryName string        `json:"country_name"`
	Flag        string        `json:"flag"`
	LastChecked time.Time     `json:"last_checked"`
}

// FormatLink returns the proxy in an import-ready URI scheme, e.g. "socks5://1.2.3.4:1080"
func (p *CheckedProxy) FormatLink() string {
	proto := p.Protocol
	if proto == "http" {
		// some prefer http://, SOCKS is socks5:// or socks4://
	}
	return fmt.Sprintf("%s://%s:%d", proto, p.IP, p.Port)
}

// CountryCodeToFlag converts ISO 3166-1 alpha-2 country code to emoji flag.
func CountryCodeToFlag(code string) string {
	if len(code) != 2 {
		return "🏳️"
	}
	code = strings.ToUpper(code)
	if code[0] < 'A' || code[0] > 'Z' || code[1] < 'A' || code[1] > 'Z' {
		return "🏳️"
	}
	// Regional Indicator Symbols start at 1F1E6 ('A')
	r1 := rune(code[0]) - 'A' + 0x1F1E6
	r2 := rune(code[1]) - 'A' + 0x1F1E6
	return string(r1) + string(r2)
}

var proxyRegex = regexp.MustCompile(`(?i)\b(?:(http|https|socks4|socks5)://)?((?:[0-9]{1,3}\.){3}[0-9]{1,3}):([0-9]{2,5})\b`)

func tryTranslateGithubURL(rawURL string) string {
	if !strings.Contains(rawURL, "raw.githubusercontent.com") {
		return rawURL
	}
	prefix := "https://raw.githubusercontent.com/"
	if !strings.HasPrefix(rawURL, prefix) {
		prefix = "http://raw.githubusercontent.com/"
		if !strings.HasPrefix(rawURL, prefix) {
			return rawURL
		}
	}

	remainder := rawURL[len(prefix):]
	parts := strings.Split(remainder, "/")
	if len(parts) < 4 {
		return rawURL
	}

	user := parts[0]
	repo := parts[1]
	branch := parts[2]
	path := strings.Join(parts[3:], "/")

	return fmt.Sprintf("https://cdn.jsdelivr.net/gh/%s/%s@%s/%s", user, repo, branch, path)
}

// ScrapeURL fetches a URL and parses IP:Port and suggested protocol.
func ScrapeURL(ctx context.Context, targetURL string) ([]RawProxy, error) {
	actualURL := tryTranslateGithubURL(targetURL)

	sctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(sctx, "GET", actualURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status code: %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	bodyStr := string(bodyBytes)
	matches := proxyRegex.FindAllStringSubmatch(bodyStr, -1)

	// Determine default suggested protocol from the URL path
	defaultProto := ""
	urlLower := strings.ToLower(targetURL)
	if strings.Contains(urlLower, "socks5") {
		defaultProto = "socks5"
	} else if strings.Contains(urlLower, "socks4") {
		defaultProto = "socks4"
	} else if strings.Contains(urlLower, "http") {
		defaultProto = "http"
	}

	var results []RawProxy
	seen := make(map[string]bool)

	for _, match := range matches {
		if len(match) < 4 {
			continue
		}
		ip := match[2]
		portStr := match[3]
		ipPort := ip + ":" + portStr

		if seen[ipPort] {
			continue
		}
		seen[ipPort] = true

		proto := strings.ToLower(match[1])
		if proto == "https" {
			proto = "http"
		}
		if proto == "" {
			proto = defaultProto
		}

		results = append(results, RawProxy{
			IPPort:        ipPort,
			SuggestedType: proto,
		})
	}

	return results, nil
}

// CheckSingleProtocol performs checking for a specific protocol.
func CheckSingleProtocol(ctx context.Context, protocol, ipPort, checkURL string, timeout time.Duration) (*CheckedProxy, error) {
	var client *http.Client

	switch protocol {
	case "socks5":
		dialer, err := proxy.SOCKS5("tcp", ipPort, nil, &net.Dialer{
			Timeout: timeout,
		})
		if err != nil {
			return nil, err
		}
		client = &http.Client{
			Transport: &http.Transport{
				DialContext: func(cctx context.Context, network, addr string) (net.Conn, error) {
					if cd, ok := dialer.(proxy.ContextDialer); ok {
						return cd.DialContext(cctx, network, addr)
					}
					return dialer.Dial(network, addr)
				},
			},
			Timeout: timeout,
		}

	case "socks4":
		client = &http.Client{
			Transport: &http.Transport{
				DialContext: func(cctx context.Context, network, addr string) (net.Conn, error) {
					return DialSOCKS4(cctx, ipPort, addr, timeout)
				},
			},
			Timeout: timeout,
		}

	case "http":
		proxyURL, err := url.Parse("http://" + ipPort)
		if err != nil {
			return nil, err
		}
		client = &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyURL(proxyURL),
				DialContext: (&net.Dialer{
					Timeout: timeout,
				}).DialContext,
			},
			Timeout: timeout,
		}

	default:
		return nil, fmt.Errorf("unsupported protocol: %s", protocol)
	}

	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, "GET", checkURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("non-200 status: %d", resp.StatusCode)
	}

	duration := time.Since(start)

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 1024))
	if err != nil {
		return nil, err
	}

	bodyStr := string(bodyBytes)
	cc, cn := "??", "Unknown"

	// Parse ip2c.org output if that's the URL
	if strings.Contains(checkURL, "ip2c.org") {
		parts := strings.Split(strings.TrimSpace(bodyStr), ";")
		if len(parts) >= 4 && parts[0] == "1" {
			cc = parts[1]
			cn = parts[3]
		}
	}

	host, portStr, _ := net.SplitHostPort(ipPort)
	port, _ := strconv.Atoi(portStr)

	return &CheckedProxy{
		IP:          host,
		Port:        port,
		Protocol:    protocol,
		Ping:        duration,
		CountryCode: cc,
		CountryName: cn,
		Flag:        CountryCodeToFlag(cc),
		LastChecked: time.Now(),
	}, nil
}

// CheckProxy attempts to check the raw proxy. If suggested protocol is set, it tests only that first.
// Otherwise, it checks in parallel or sequentially.
func CheckProxy(ctx context.Context, raw RawProxy, checkURL string, timeout time.Duration, allowedProtos []string) (*CheckedProxy, error) {
	// If a protocol is suggested and allowed, try it first
	if raw.SuggestedType != "" {
		allowed := false
		for _, p := range allowedProtos {
			if p == raw.SuggestedType {
				allowed = true
				break
			}
		}
		if allowed {
			res, err := CheckSingleProtocol(ctx, raw.SuggestedType, raw.IPPort, checkURL, timeout)
			if err == nil {
				return res, nil
			}
			// If suggested fails, we can fall back to checking others if we want, or just return error.
			// Let's check others as a fallback to be thorough.
		}
	}

	// Try all allowed protocols concurrently
	type checkRes struct {
		proxy *CheckedProxy
		err   error
	}

	resChan := make(chan checkRes, len(allowedProtos))
	cctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	for _, proto := range allowedProtos {
		if proto == raw.SuggestedType {
			continue // Already checked
		}
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			res, err := CheckSingleProtocol(cctx, p, raw.IPPort, checkURL, timeout)
			if err == nil {
				// Cancel other checks immediately
				cancel()
				resChan <- checkRes{proxy: res, err: nil}
			} else {
				resChan <- checkRes{proxy: nil, err: err}
			}
		}(proto)
	}

	// Wait in background to close channel
	go func() {
		wg.Wait()
		close(resChan)
	}()

	var lastErr error
	for r := range resChan {
		if r.err == nil {
			return r.proxy, nil
		}
		lastErr = r.err
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("all protocols failed")
}
