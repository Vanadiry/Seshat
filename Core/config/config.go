// Package config 管理 seshatd 的 TOML 配置文件读取、写入和默认值。
// 配置文件默认位于 ~/.vSoft/Seshat/config.toml，可通过 SESHAT_HOME 环境变量覆盖。
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

var Defaults = Config{
	BindAddr:    "127.0.0.1",
	Port:        4000,
	DataHome:    "",
	Username:    "",
	SyncEnabled: false,
	Concurrency: 32,
	BaseURL:     "https://api.bgm.tv",
}

type Config struct {
	BindAddr     string `toml:"bind_addr"`
	Port         int    `toml:"port"`
	DataHome     string `toml:"data_home"`
	Username     string `toml:"username"`
	SyncEnabled  bool   `toml:"sync_enabled"`
	Concurrency  int    `toml:"concurrency"`
	BaseURL      string `toml:"base_url"`
}

func Dir() string {
	if d := os.Getenv("SESHAT_HOME"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".vSoft", "Seshat")
}

func Path() string {
	dir := Dir()
	tomlPath := filepath.Join(dir, "config.toml")
	if _, err := os.Stat(tomlPath); err == nil {
		return tomlPath
	}
	return tomlPath
}

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
	if cfg.Port < 1 || cfg.Port > 65535 {
		cfg.Port = Defaults.Port
	}
	if cfg.Concurrency < 1 || cfg.Concurrency > 128 {
		cfg.Concurrency = Defaults.Concurrency
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
	if cfg.Port < 1 || cfg.Port > 65535 {
		return fmt.Errorf("port 必须在 1-65535 之间")
	}
	if cfg.Concurrency < 1 || cfg.Concurrency > 128 {
		return fmt.Errorf("concurrency 必须在 1-128 之间")
	}
	return nil
}

func (c *Config) DataDir() string {
	if c.DataHome != "" {
		return c.DataHome
	}
	return filepath.Join(Dir(), "data")
}

// TrackerDir 返回 tracker 文件目录。
func (c *Config) TrackerDir() string {
	return filepath.Join(Dir(), "tracker")
}
