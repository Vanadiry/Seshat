package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

// TestVersionConsistent 检查各处声明的版本是否与 Version 常量一致
func TestVersionConsistent(t *testing.T) {
	root := filepath.Join("..", "..")
	got := map[string]string{"core/config.Version": Version}

	var tauri struct {
		Version string `json:"version"`
	}
	readJSONFile(t, filepath.Join(root, "src-tauri", "tauri.conf.json"), &tauri)
	got["src-tauri/tauri.conf.json"] = tauri.Version

	var cargo struct {
		Package struct {
			Version string `toml:"version"`
		} `toml:"package"`
	}
	readTOMLFile(t, filepath.Join(root, "src-tauri", "Cargo.toml"), &cargo)
	got["src-tauri/Cargo.toml"] = cargo.Package.Version

	var lock struct {
		Package []struct {
			Name    string `toml:"name"`
			Version string `toml:"version"`
		} `toml:"package"`
	}
	readTOMLFile(t, filepath.Join(root, "src-tauri", "Cargo.lock"), &lock)
	found := false
	for _, p := range lock.Package {
		if p.Name == "Seshat" {
			got["src-tauri/Cargo.lock"] = p.Version
			found = true
			break
		}
	}
	if !found {
		t.Fatal("Cargo.lock 中未找到 Seshat 包")
	}

	data, err := os.ReadFile(filepath.Join(root, "docs", "api", "openapi.yaml"))
	if err != nil {
		t.Fatalf("读取 openapi.yaml: %v", err)
	}
	got["docs/api/openapi.yaml"] = openapiVersion(string(data))

	for name, v := range got {
		if v != Version {
			t.Errorf("%s 的版本为 %q，期望 %q", name, v, Version)
		}
	}
}

func readJSONFile(t *testing.T, path string, v any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取 %s: %v", path, err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("解析 %s: %v", path, err)
	}
}

func readTOMLFile(t *testing.T, path string, v any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取 %s: %v", path, err)
	}
	if err := toml.Unmarshal(data, v); err != nil {
		t.Fatalf("解析 %s: %v", path, err)
	}
}

var openapiVersionRE = regexp.MustCompile(`(?m)^\s{2}version:\s*"([^"]+)"`)

func openapiVersion(data string) string {
	m := openapiVersionRE.FindStringSubmatch(data)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}
