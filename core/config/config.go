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

func Dir() string {
	if d := os.Getenv("SESHAT_HOME"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".vSoft", "Seshat")
}

func Path() string { return filepath.Join(Dir(), "config.toml") }

// Load 读取 config.toml；不存在时按 default.go 生成一份
func Load() (*Config, error) {
	path := Path()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			os.MkdirAll(Dir(), 0o755)
			data = []byte(DefaultConfigTOML)
			os.WriteFile(path, data, 0o644)
		} else {
			return nil, fmt.Errorf("读取配置失败: %w", err)
		}
	}
	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}
	return &cfg, nil
}

// Validate 校验会影响程序正常运行的字段。返回错误则不应启动。
func (c *Config) Validate() error {
	if c.Server.BindAddr == "" {
		return fmt.Errorf("bind_addr 不能为空")
	}
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("port 必须在 1-65535 之间（当前 %d）", c.Server.Port)
	}
	if c.Server.ConcurrencyInfo < 1 {
		return fmt.Errorf("concurrency_info 必须 >= 1（当前 %d）", c.Server.ConcurrencyInfo)
	}
	if c.Server.ConcurrencyImage < 1 {
		return fmt.Errorf("concurrency_image 必须 >= 1（当前 %d）", c.Server.ConcurrencyImage)
	}
	return nil
}

// Warnings 返回不致命、但会影响部分功能的配置问题。
func (c *Config) Warnings() []string {
	w := []string{}
	if c.Upstream.BaseURL == "" {
		w = append(w, "未配置 base_url，无法拉取数据")
	}
	if c.Upstream.UserAgent == "" {
		w = append(w, "未设置 user_agent，上游可能拒绝请求")
	}
	return w
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

func (c *Config) DataDir() string {
	if c.Server.DataHome != "" {
		return c.Server.DataHome
	}
	return filepath.Join(Dir(), "data")
}

func (c *Config) TrackerDir() string { return filepath.Join(Dir(), "tracker") }
