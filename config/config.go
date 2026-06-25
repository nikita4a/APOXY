package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Threads       int           `yaml:"threads"`
	Timeout       time.Duration `yaml:"timeout"`
	CheckURL      string        `yaml:"check_url"`
	ExportPath    string        `yaml:"export_path"`
	Sources       []string      `yaml:"sources"`
	Protocols     []string      `yaml:"protocols"` // "http", "socks4", "socks5"
}

func DefaultConfig() *Config {
	return &Config{
		Threads:    150,
		Timeout:    time.Second * 6,
		CheckURL:   "http://ip2c.org/self",
		ExportPath: "exports/live_proxies.txt",
		Protocols:  []string{"http", "socks4", "socks5"},
		Sources: []string{
			// TheSpeedX public lists
			"https://cdn.jsdelivr.net/gh/TheSpeedX/PROXY-List@master/http.txt",
			"https://cdn.jsdelivr.net/gh/TheSpeedX/PROXY-List@master/socks4.txt",
			"https://cdn.jsdelivr.net/gh/TheSpeedX/PROXY-List@master/socks5.txt",
			// monosans lists
			"https://cdn.jsdelivr.net/gh/monosans/proxy-list@main/proxies/http.txt",
			"https://cdn.jsdelivr.net/gh/monosans/proxy-list@main/proxies/socks4.txt",
			"https://cdn.jsdelivr.net/gh/monosans/proxy-list@main/proxies/socks5.txt",
			// hookzof SOCKS5 list
			"https://cdn.jsdelivr.net/gh/hookzof/socks5-list@master/proxy.txt",
			// clarketm list
			"https://cdn.jsdelivr.net/gh/clarketm/proxy-list@master/proxy-list-raw.txt",
		},
	}
}

func LoadConfig(path string) (*Config, error) {
	cfg := DefaultConfig()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Save default config
			return cfg, cfg.Save(path)
		}
		return nil, err
	}

	err = yaml.Unmarshal(data, cfg)
	if err != nil {
		return nil, err
	}

	if cfg.Threads <= 0 {
		cfg.Threads = 50
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = time.Second * 5
	}
	if cfg.CheckURL == "" {
		cfg.CheckURL = "http://ip2c.org/self"
	}
	if cfg.ExportPath == "" {
		cfg.ExportPath = "exports/live_proxies.txt"
	}

	return cfg, nil
}

func (c *Config) Save(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	// Ensure directory exists
	dir := path
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			dir = path[:i]
			break
		}
	}
	if dir != path {
		_ = os.MkdirAll(dir, 0755)
	}
	return os.WriteFile(path, data, 0644)
}
