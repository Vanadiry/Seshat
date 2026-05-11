package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

var Defaults = Config{
	BindAddr:    "127.0.0.1",
	Port:        8080,
	DataHome:    "",
	Username:    "",
	Concurrency: 32,
}

type Config struct {
	BindAddr    string `toml:"bind_addr"`
	Port        int    `toml:"port"`
	DataHome    string `toml:"data_home"`
	Username    string `toml:"username"`
	Concurrency int    `toml:"concurrency"`
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
