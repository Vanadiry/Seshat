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

// prefsDefaults 偏好项的默认值
var prefsDefaults = map[string]string{
	"theme":           "auto",
	"prefer_lang":     "original",
	"username":        "",
	"subject_sort":    "elo",
	"auto_link_names": "true",
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
	for k, v := range prefsDefaults {
		defMap[k] = v
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

// BuildPrefsKV 返回偏好纯 KV，未设置则用默认值
func BuildPrefsKV() map[string]string {
	m, _ := loadOverrides()
	if m == nil {
		m = map[string]string{}
	}
	result := map[string]string{}
	for k, def := range prefsDefaults {
		v, ok := m[k]
		if !ok || v == "" {
			v = def
		}
		result[k] = v
	}
	return result
}

// ApplyOverrides 合并设置更新到覆盖文件，空白或默认值则删除该项
func ApplyOverrides(updates map[string]any) (okList, failList []string) {
	valid := map[string]string{}
	for k, v := range prefsDefaults {
		valid[k] = v
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
