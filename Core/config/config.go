// Package config 管理 TOML 配置文件读取、写入和默认值
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type ServerConfig struct {
	BindAddr         string `toml:"bind_addr"`
	Port             int    `toml:"port"`
	ConcurrencyInfo  int    `toml:"concurrency_info"`
	ConcurrencyImage int    `toml:"concurrency_image"`
	DataHome         string `toml:"data_home"`
	LogLevel         string `toml:"log_level"`
}

type UpstreamConfig struct {
	BaseURL   string `toml:"base_url"`
	UserAgent string `toml:"user_agent"`
}

type FrontendConfig struct {
	BackendURL  string `toml:"backend_url"`
	FallbackURL string `toml:"fallback_url"`
}

type AccessConfig struct {
	Token string `toml:"bangumi_access_token"`
}

type Config struct {
	Server   ServerConfig   `toml:"server"`
	Upstream UpstreamConfig `toml:"upstream"`
	Frontend FrontendConfig `toml:"frontend"`
	Access   AccessConfig   `toml:"access"`
}

var Defaults = Config{
	Server: ServerConfig{
		BindAddr:         "127.0.0.1",
		Port:             12500,
		ConcurrencyInfo:  4,
		ConcurrencyImage: 16,
		DataHome:         "",
		LogLevel:         "warn",
	},
	Upstream: UpstreamConfig{
		BaseURL:   "https://api.bgm.tv",
		UserAgent: "Vanadiry/Seshat/v1.3.0 (https://github.com/Vanadiry/Seshat)",
	},
	Frontend: FrontendConfig{BackendURL: "", FallbackURL: ""},
}

func Dir() string {
	if d := os.Getenv("SESHAT_HOME"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".vSoft", "Seshat")
}

func Path() string { return filepath.Join(Dir(), "config.toml") }

func Load() (*Config, error) {
	cfg := Defaults
	path := Path()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			os.MkdirAll(Dir(), 0o755)
			os.WriteFile(path, []byte(DefaultConfigTOML), 0o644)
			return &cfg, nil
		}
		return nil, fmt.Errorf("读取配置失败: %w", err)
	}
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}
	if cfg.Server.Port < 1 || cfg.Server.Port > 65535 {
		cfg.Server.Port = Defaults.Server.Port
	}
	if cfg.Server.ConcurrencyInfo < 1 {
		cfg.Server.ConcurrencyInfo = Defaults.Server.ConcurrencyInfo
	}
	if cfg.Server.ConcurrencyImage < 1 {
		cfg.Server.ConcurrencyImage = Defaults.Server.ConcurrencyImage
	}
	if cfg.Upstream.UserAgent == "" {
		cfg.Upstream.UserAgent = Defaults.Upstream.UserAgent
	}
	return &cfg, nil
}

// BuildConfigKV 返回 config.toml 当前值的纯 KV，token 非空显示 ***
func (c *Config) BuildConfigKV() map[string]any {
	token := c.Access.Token
	if len(token) > 6 {
		token = token[:3] + "***" + token[len(token)-3:]
	} else if token != "" {
		token = "***"
	}
	return map[string]any{
		"bind_addr":            c.Server.BindAddr,
		"port":                 c.Server.Port,
		"concurrency_info":     c.Server.ConcurrencyInfo,
		"concurrency_image":    c.Server.ConcurrencyImage,
		"data_home":            c.Server.DataHome,
		"log_level":            c.Server.LogLevel,
		"base_url":             c.Upstream.BaseURL,
		"user_agent":           c.Upstream.UserAgent,
		"backend_url":          c.Frontend.BackendURL,
		"fallback_url":         c.Frontend.FallbackURL,
		"bangumi_access_token": token,
	}
}

func Save(cfg *Config) error {
	dir := Dir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(dir, "config.toml"))
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(cfg)
}

func Validate(cfg *Config) error {
	if cfg.Server.Port < 1 || cfg.Server.Port > 65535 {
		return fmt.Errorf("port 必须在 1-65535 之间")
	}
	if cfg.Server.ConcurrencyInfo < 1 {
		return fmt.Errorf("concurrency_info 必须 >= 1")
	}
	if cfg.Server.ConcurrencyImage < 1 {
		return fmt.Errorf("concurrency_image 必须 >= 1")
	}
	return nil
}

func (c *Config) DataDir() string {
	if c.Server.DataHome != "" {
		return c.Server.DataHome
	}
	return filepath.Join(Dir(), "data")
}

func (c *Config) TrackerDir() string { return filepath.Join(Dir(), "tracker") }
