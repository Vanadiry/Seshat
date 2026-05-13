// Package config 管理 seshatd 的 TOML 配置文件读取、写入和默认值。
// 配置文件默认位于 ~/.vSoft/Seshat/config.toml，可通过 SESHAT_HOME 环境变量覆盖。
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type ServerConfig struct {
	BindAddr    string `toml:"bind_addr"`
	Port        int    `toml:"port"`
	Concurrency int    `toml:"concurrency"`
	DataHome    string `toml:"data_home"`
}

type UpstreamConfig struct {
	BaseURL   string `toml:"base_url"`
	UserAgent string `toml:"user_agent"`
}

type FrontendConfig struct {
	BackendURL  string `toml:"backend_url"`
	PreferLang  string `toml:"prefer_lang"`
	FallbackURL string `toml:"fallback_url"`
}

type UserConfig struct {
	Username string `toml:"username"`
}

type Config struct {
	Server   ServerConfig   `toml:"server"`
	Upstream UpstreamConfig `toml:"upstream"`
	Frontend FrontendConfig `toml:"frontend"`
	User     UserConfig     `toml:"user"`
}

var Defaults = Config{
	Server: ServerConfig{
		BindAddr:    "127.0.0.1",
		Port:        4000,
		Concurrency: 32,
		DataHome:    "",
	},
	Upstream: UpstreamConfig{
		BaseURL:   "https://api.bgm.tv",
		UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36",
	},
	Frontend: FrontendConfig{BackendURL: "", PreferLang: "original", FallbackURL: ""},
	User:     UserConfig{Username: ""},
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
	if cfg.Server.Concurrency < 1 {
		cfg.Server.Concurrency = Defaults.Server.Concurrency
	}
	if cfg.Upstream.UserAgent == "" {
		cfg.Upstream.UserAgent = Defaults.Upstream.UserAgent
	}
	return &cfg, nil
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
	if cfg.Server.Concurrency < 1 {
		return fmt.Errorf("concurrency 必须 >= 1")
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
