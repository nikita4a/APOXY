package proxy

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type ExportFilter struct {
	Protocols []string
	MaxPing   time.Duration
	Countries []string // ISO codes, upper case
}

func (ef *ExportFilter) Match(p *CheckedProxy) bool {
	if p == nil {
		return false
	}

	// Filter by Protocol
	if len(ef.Protocols) > 0 {
		matchedProto := false
		for _, proto := range ef.Protocols {
			if strings.ToLower(p.Protocol) == strings.ToLower(proto) {
				matchedProto = true
				break
			}
		}
		if !matchedProto {
			return false
		}
	}

	// Filter by Max Ping
	if ef.MaxPing > 0 && p.Ping > ef.MaxPing {
		return false
	}

	// Filter by Country
	if len(ef.Countries) > 0 {
		matchedCountry := false
		pcc := strings.ToUpper(p.CountryCode)
		for _, c := range ef.Countries {
			if strings.ToUpper(c) == pcc {
				matchedCountry = true
				break
			}
		}
		if !matchedCountry {
			return false
		}
	}

	return true
}

func Export(proxies []*CheckedProxy, filter ExportFilter, format string, targetPath string) (int, error) {
	// Create directory if not exists
	dir := filepath.Dir(targetPath)
	if dir != "." && dir != "/" {
		_ = os.MkdirAll(dir, 0755)
	}

	file, err := os.Create(targetPath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	var filtered []*CheckedProxy
	for _, p := range proxies {
		if filter.Match(p) {
			filtered = append(filtered, p)
		}
	}

	switch strings.ToLower(format) {
	case "raw":
		for _, p := range filtered {
			_, _ = fmt.Fprintf(file, "%s:%d\n", p.IP, p.Port)
		}

	case "uri":
		for _, p := range filtered {
			_, _ = fmt.Fprintf(file, "%s\n", p.FormatLink())
		}

	case "pretty":
		_, _ = fmt.Fprintf(file, "MINOXY PROXY REPORT - %s\n", time.Now().Format(time.RFC1123))
		_, _ = fmt.Fprintf(file, "Total Live Proxies: %d\n", len(filtered))
		_, _ = fmt.Fprintf(file, "%-6s | %-22s | %-8s | %-6s | %s\n", "Flag", "IP:Port", "Protocol", "Ping", "Country")
		_, _ = fmt.Fprintf(file, "%s\n", strings.Repeat("-", 70))
		for _, p := range filtered {
			ipPort := fmt.Sprintf("%s:%d", p.IP, p.Port)
			pingMs := fmt.Sprintf("%dms", p.Ping.Milliseconds())
			_, _ = fmt.Fprintf(file, "%-6s | %-22s | %-8s | %-6s | %s (%s)\n",
				p.Flag, ipPort, p.Protocol, pingMs, p.CountryName, p.CountryCode)
		}

	case "csv":
		writer := csv.NewWriter(file)
		_ = writer.Write([]string{"IP", "Port", "Protocol", "PingMs", "CountryCode", "CountryName", "URI"})
		for _, p := range filtered {
			_ = writer.Write([]string{
				p.IP,
				strconv.Itoa(p.Port),
				p.Protocol,
				strconv.FormatInt(p.Ping.Milliseconds(), 10),
				p.CountryCode,
				p.CountryName,
				p.FormatLink(),
			})
		}
		writer.Flush()

	case "json":
		encoder := json.NewEncoder(file)
		encoder.SetIndent("", "  ")
		_ = encoder.Encode(filtered)

	default:
		return 0, fmt.Errorf("unknown export format: %s", format)
	}

	return len(filtered), nil
}
