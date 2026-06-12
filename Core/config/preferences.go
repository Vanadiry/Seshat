package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Preferences 运行时的扁平偏好结构
type Preferences struct {
	Theme         string
	PreferLang    string
	Username      string
	SubjectSort   string
	AutoLinkNames string
}

// SettingChoice 偏好选项
type SettingChoice struct {
	Value string `json:"-"`
	Label string `json:"-"`
}

// SettingDef 单个偏好项定义
type SettingDef struct {
	Key     string
	Comment string
	Choices []SettingChoice
	Default string
}

// allSettings 全部偏好项的定义
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
		Key: "username", Comment: "Bangumi 用户 ID，用于拉取收藏和头像",
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
	{
		Key: "auto_link_names", Comment: "详情页自动高亮角色和人物名称",
		Choices: []SettingChoice{
			{Value: "true", Label: "开启"},
			{Value: "false", Label: "关闭"},
		},
		Default: "true",
	},
}

var DefaultPreferences = Preferences{
	PreferLang:    "original",
	SubjectSort:   "elo",
	AutoLinkNames: "true",
}

func PrefDir() string  { return filepath.Join(Dir(), "user", "settings") }
func PrefPath() string { return filepath.Join(PrefDir(), "preferences.json") }

// loadOverrides 读取稀疏覆盖文件 {key: value}
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

// saveOverrides 写入稀疏覆盖文件
func saveOverrides(m map[string]string) error {
	os.MkdirAll(PrefDir(), 0o755)
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(PrefPath(), data, 0o644)
}

// LoadPreferences 读取偏好，缺失项使用默认值
func LoadPreferences() (*Preferences, error) {
	m, err := loadOverrides()
	if err != nil {
		if os.IsNotExist(err) {
			return &DefaultPreferences, nil
		}
		return nil, err
	}

	defMap := map[string]string{}
	for _, def := range allSettings {
		defMap[def.Key] = def.Default
	}
	for k, v := range m {
		if _, ok := defMap[k]; ok {
			defMap[k] = v
		}
	}

	return &Preferences{
		Theme:         defMap["theme"],
		PreferLang:    defMap["prefer_lang"],
		Username:      defMap["username"],
		SubjectSort:   defMap["subject_sort"],
		AutoLinkNames: defMap["auto_link_names"],
	}, nil
}

// BuildSettingsJSON 生成发往前端的完整设置 JSON
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

// ApplyOverrides 合并设置更新到覆盖文件，空白或默认值则删除该项
func ApplyOverrides(updates map[string]any) (okList, failList []string) {
	valid := map[string]string{}
	for _, def := range allSettings {
		valid[def.Key] = def.Default
	}

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
			delete(m, k)
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
