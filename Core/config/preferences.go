package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Preferences is the flat runtime representation of user preferences.
type Preferences struct {
	Theme       string
	PreferLang  string
	Username    string
	SubjectSort string
}

// SettingChoice represents an option in a preference dropdown.
type SettingChoice struct {
	Value string `json:"-"`
	Label string `json:"-"`
}

// SettingDef defines a single preference item.
type SettingDef struct {
	Key     string
	Comment string
	Choices []SettingChoice
	Default string
}

// allSettings is the single source of truth for all preferences.
var allSettings = []SettingDef{
	{
		Key: "theme", Comment: "界面主题",
		Choices: []SettingChoice{
			{Value: "auto", Label: "同步系统"},
			{Value: "dark", Label: "深色"},
			{Value: "light", Label: "浅色"},
		},
		Default: "auto",
	},
	{
		Key: "prefer_lang", Comment: "标题展示方式",
		Choices: []SettingChoice{
			{Value: "original", Label: "原文优先"},
			{Value: "chinese", Label: "中文优先"},
		},
		Default: "original",
	},
	{
		Key: "username", Comment: "在此处填写你的 Bangumi番组计划 ID，将能够拉取收藏和头像等信息",
		Default: "",
	},
	{
		Key: "subject_sort", Comment: "首页条目排序方式",
		Choices: []SettingChoice{
			{Value: "elo", Label: "ELO Rating"},
			{Value: "bgm_rank", Label: "BGM Rank"},
			{Value: "random", Label: "随机"},
			{Value: "none", Label: "不排序"},
		},
		Default: "elo",
	},
}

var DefaultPreferences = Preferences{
	PreferLang:  "original",
	SubjectSort: "elo",
}

func PrefDir() string  { return filepath.Join(Dir(), "user", "settings") }
func PrefPath() string { return filepath.Join(PrefDir(), "preferences.json") }

// loadOverrides reads the sparse overrides file (just {key: value}).
func loadOverrides() (map[string]string, error) {
	data, err := os.ReadFile(PrefPath())
	if err != nil {
		return nil, err
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// saveOverrides writes the sparse overrides file.
func saveOverrides(m map[string]string) error {
	os.MkdirAll(PrefDir(), 0o755)
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(PrefPath(), data, 0o644)
}

// migrateOldPrefs moves settings/preferences.json to user/settings/preferences.json
// and converts from old structured format to sparse KV.
func migrateOldPrefs() {
	oldPath := filepath.Join(Dir(), "settings", "preferences.json")
	data, err := os.ReadFile(oldPath)
	if err != nil {
		return
	}
	var old map[string]any
	if json.Unmarshal(data, &old) != nil {
		return
	}
	m := map[string]string{}
	for _, def := range allSettings {
		if entry, ok := old[def.Key].(map[string]any); ok {
			if v, ok := entry["value"].(string); ok && v != "" && v != def.Default {
				m[def.Key] = v
			}
		}
	}
	if len(m) == 0 {
		return
	}
	saveOverrides(m)
	os.Remove(oldPath)
}

// LoadPreferences returns user preferences, filling defaults where no override exists.
func LoadPreferences() (*Preferences, error) {
	migrateOldPrefs()

	m, err := loadOverrides()
	if err != nil {
		if os.IsNotExist(err) {
			return &DefaultPreferences, nil
		}
		return nil, err
	}

	// Fill defaults
	defMap := map[string]string{}
	for _, def := range allSettings {
		defMap[def.Key] = def.Default
	}
	// Apply overrides
	for k, v := range m {
		if _, ok := defMap[k]; ok {
			defMap[k] = v
		}
	}

	return &Preferences{
		Theme:       defMap["theme"],
		PreferLang:  defMap["prefer_lang"],
		Username:    defMap["username"],
		SubjectSort: defMap["subject_sort"],
	}, nil
}

// BuildSettingsJSON generates the full settings JSON sent to the frontend.
func BuildSettingsJSON() map[string]any {
	m, _ := loadOverrides()
	if m == nil {
		m = map[string]string{}
	}
	result := map[string]any{}
	for _, def := range allSettings {
		entry := map[string]any{}
		if len(def.Choices) > 0 {
			choices := make([]map[string]string, len(def.Choices))
			for i, c := range def.Choices {
				choices[i] = map[string]string{c.Value: c.Label}
			}
			entry["_choice"] = choices
		}
		if def.Comment != "" {
			entry["_comment"] = def.Comment
		}
		v, ok := m[def.Key]
		if !ok || v == "" {
			v = def.Default
		}
		entry["value"] = v
		result[def.Key] = entry
	}
	return result
}

// ApplyOverrides merges the given key-value pairs into the overrides file.
// Only keys defined in allSettings are accepted; blank/default values are removed.
func ApplyOverrides(updates map[string]any) (okList, failList []string) {
	// Build lookup of valid keys
	valid := map[string]string{} // key → default
	for _, def := range allSettings {
		valid[def.Key] = def.Default
	}

	// Load existing overrides
	m, _ := loadOverrides()
	if m == nil {
		m = map[string]string{}
	}

	for k, v := range updates {
		def, exists := valid[k]
		if !exists {
			failList = append(failList, k)
			continue
		}
		s, ok := v.(string)
		if !ok {
			failList = append(failList, k)
			continue
		}
		if s == "" || s == def {
			delete(m, k) // remove override, use default
		} else {
			m[k] = s
		}
		okList = append(okList, k)
	}

	if len(m) == 0 {
		os.Remove(PrefPath())
	} else {
		saveOverrides(m)
	}
	return
}
