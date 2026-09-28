package config

import (
	"encoding/json"
	"os"
	"sync"
)

type Config struct {
	AccessToken string  `json:"access_token"`
	Phone       string  `json:"phone"`
	UnitName    string  `json:"unit_name"`
	MinChunkSec float64 `json:"min_chunk_sec"`
	MaxChunkSec float64 `json:"max_chunk_sec"`
}

var (
	configFile = "config.json"
	mu         sync.Mutex
)

// DefaultConfig 获取默认配置
func DefaultConfig() *Config {
	return &Config{
		AccessToken: "",
		Phone:       "",
		UnitName:    "",
		MinChunkSec: 9.7,
		MaxChunkSec: 10.3,
	}
}

// Load 从当前目录加载 config.json，若不存在则创建默认配置
func Load() *Config {
	mu.Lock()
	defer mu.Unlock()

	cfg := DefaultConfig()
	data, err := os.ReadFile(configFile)
	if err != nil {
		return cfg
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return cfg
	}

	if cfg.MinChunkSec <= 0 {
		cfg.MinChunkSec = 9.7
	}
	if cfg.MaxChunkSec <= cfg.MinChunkSec {
		cfg.MaxChunkSec = cfg.MinChunkSec + 0.6
	}

	return cfg
}

// Save 保存配置到 config.json
func Save(cfg *Config) error {
	mu.Lock()
	defer mu.Unlock()

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configFile, data, 0644)
}
