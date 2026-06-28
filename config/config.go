package config

import (
	"os"
	"time"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Threads     int           `yaml:"threads"`
	Timeout     time.Duration `yaml:"timeout"`
	CheckModels bool          `yaml:"check_models"`
	ExportPath  string        `yaml:"export_path"`
	Sources     []string      `yaml:"sources"`
}

func DefaultConfig() *Config {
	return &Config{Threads: 200, Timeout: 8 * time.Second, CheckModels: true, ExportPath: "exports/api_proxies.json", Sources: []string{}}
}

func LoadConfig(path string) (*Config, error) {
	cfg := DefaultConfig()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) { return cfg, cfg.Save(path) }
		return nil, err
	}
	yaml.Unmarshal(data, cfg)
	if cfg.Threads <= 0 { cfg.Threads = 50 }
	if cfg.Timeout <= 0 { cfg.Timeout = 5 * time.Second }
	if cfg.ExportPath == "" { cfg.ExportPath = "exports/api_proxies.json" }
	return cfg, nil
}

func (c *Config) Save(path string) error {
	data, _ := yaml.Marshal(c)
	return os.WriteFile(path, data, 0644)
}
