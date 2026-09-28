package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	mrand "math/rand"
	"os"
	"sync"
	"time"
)

type Config struct {
	AccessToken       string  `json:"access_token"`
	Phone             string  `json:"phone"`
	UnitName          string  `json:"unit_name"`
	UnitID            int64   `json:"unit_id"`
	UserID            int64   `json:"user_id"`
	DeviceNumber      string  `json:"device_number"`
	ManufacturerBrand string  `json:"manufacturer_brand"`
	JPushDeviceToken  string  `json:"jpush_device_token"`
	DeviceModel       string  `json:"device_model"`
	SysVersion        string  `json:"sys_version"`
	CodeVersion       string  `json:"code_version"`
	MinChunkSec       float64 `json:"min_chunk_sec"`
	MaxChunkSec       float64 `json:"max_chunk_sec"`
}

var (
	configFile = "config.json"
	mu         sync.Mutex
)

func randomBytes(n int) []byte {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		r := mrand.New(mrand.NewSource(time.Now().UnixNano()))
		r.Read(b)
	}
	return b
}

func generateUUID() string {
	b := randomBytes(16)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func generateJPushToken() string {
	return "190e35f7" + hex.EncodeToString(randomBytes(6))
}

// DefaultConfig 获取默认配置，自动生成独立的设备指纹并持久化
func DefaultConfig() *Config {
	return &Config{
		AccessToken:       "",
		Phone:             "",
		UnitName:          "",
		UnitID:            0,
		UserID:            0,
		DeviceNumber:      generateUUID(),
		ManufacturerBrand: "huawei",
		JPushDeviceToken:  generateJPushToken(),
		DeviceModel:       "android",
		SysVersion:        "12",
		CodeVersion:       "7.6.9",
		MinChunkSec:       9.7,
		MaxChunkSec:       10.3,
	}
}

// Load 从当前目录加载 config.json，若缺失字段自动补齐并持久化
func Load() *Config {
	mu.Lock()
	defer mu.Unlock()

	cfg := DefaultConfig()
	data, err := os.ReadFile(configFile)
	if err == nil {
		_ = json.Unmarshal(data, cfg)
	}

	needsSave := false

	if cfg.DeviceNumber == "" {
		cfg.DeviceNumber = generateUUID()
		needsSave = true
	}
	if cfg.ManufacturerBrand == "" {
		cfg.ManufacturerBrand = "huawei"
		needsSave = true
	}
	if cfg.JPushDeviceToken == "" {
		cfg.JPushDeviceToken = generateJPushToken()
		needsSave = true
	}
	if cfg.DeviceModel == "" {
		cfg.DeviceModel = "android"
		needsSave = true
	}
	if cfg.SysVersion == "" {
		cfg.SysVersion = "12"
		needsSave = true
	}
	if cfg.CodeVersion == "" {
		cfg.CodeVersion = "7.6.9"
		needsSave = true
	}
	if cfg.MinChunkSec <= 0 {
		cfg.MinChunkSec = 9.7
		needsSave = true
	}
	if cfg.MaxChunkSec <= cfg.MinChunkSec {
		cfg.MaxChunkSec = cfg.MinChunkSec + 0.6
		needsSave = true
	}

	if needsSave || err != nil {
		_ = saveLocked(cfg)
	}

	return cfg
}

// Save 保存配置到 config.json
func Save(cfg *Config) error {
	mu.Lock()
	defer mu.Unlock()
	return saveLocked(cfg)
}

func saveLocked(cfg *Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configFile, data, 0644)
}
