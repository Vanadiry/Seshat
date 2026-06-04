package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Preferences is the flat runtime representation of user preferences.
type Preferences struct {
	Theme      string
	PreferLang string
	Username   string
}

type prefString struct {
	Comment string `json:"_comment"`
	Value   string `json:"value"`
}

type preferencesFile struct {
	Comment    string     `json:"_comment"`
	Theme      prefString `json:"theme"`
	PreferLang prefString `json:"prefer_lang"`
	Username   prefString `json:"username"`
}

var DefaultPreferences = Preferences{
	PreferLang: "original",
}

func PrefDir() string { return filepath.Join(Dir(), "settings") }

func PrefPath() string { return filepath.Join(PrefDir(), "preferences.json") }

func LoadPreferences() (*Preferences, error) {
	data, err := os.ReadFile(PrefPath())
	if err != nil {
		if os.IsNotExist(err) {
			os.MkdirAll(PrefDir(), 0o755)
			os.WriteFile(PrefPath(), []byte(DefaultPreferencesJSON), 0o644)
			return &DefaultPreferences, nil
		}
		return nil, err
	}
	var f preferencesFile
	if json.Unmarshal(data, &f) != nil {
		return &DefaultPreferences, nil
	}
	return &Preferences{
		Theme:      f.Theme.Value,
		PreferLang: f.PreferLang.Value,
		Username:   f.Username.Value,
	}, nil
}
