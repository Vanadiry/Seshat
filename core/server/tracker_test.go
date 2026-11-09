package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type seshatTrackerFile struct {
	Name     string `json:"name"`
	Subjects []int  `json:"subjects"`
}

func readSeshatTracker(t *testing.T, home string) (seshatTrackerFile, []byte) {
	t.Helper()
	path := filepath.Join(home, "tracker", "_seshat.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取 _seshat.json: %v", err)
	}
	var data seshatTrackerFile
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatalf("解析 _seshat.json: %v", err)
	}
	return data, raw
}

func TestAddToSeshatTrackerCreatesFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SESHAT_HOME", home)

	addToSeshatTracker(51)

	data, _ := readSeshatTracker(t, home)
	if data.Name != "seshat" {
		t.Fatalf("默认 name = %q, 期望 seshat", data.Name)
	}
	if len(data.Subjects) != 1 || data.Subjects[0] != 51 {
		t.Fatalf("subjects = %v, 期望 [51]", data.Subjects)
	}
}

func TestAddToSeshatTrackerDedupesAndAppends(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SESHAT_HOME", home)

	addToSeshatTracker(51)
	addToSeshatTracker(51)
	addToSeshatTracker(288)

	data, _ := readSeshatTracker(t, home)
	if len(data.Subjects) != 2 || data.Subjects[0] != 51 || data.Subjects[1] != 288 {
		t.Fatalf("subjects = %v, 期望 [51 288]", data.Subjects)
	}
}

func TestAddToSeshatTrackerKeepsCustomName(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SESHAT_HOME", home)

	dir := filepath.Join(home, "tracker")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "_seshat.json")
	if err := os.WriteFile(path, []byte(`{"name":"custom","subjects":[1]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	addToSeshatTracker(2)

	data, _ := readSeshatTracker(t, home)
	if data.Name != "custom" {
		t.Fatalf("name = %q, 期望 custom", data.Name)
	}
	if len(data.Subjects) != 2 || data.Subjects[0] != 1 || data.Subjects[1] != 2 {
		t.Fatalf("subjects = %v, 期望 [1 2]", data.Subjects)
	}
}

func TestAddToSeshatTrackerKeepsBrokenFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SESHAT_HOME", home)

	dir := filepath.Join(home, "tracker")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "_seshat.json")
	broken := []byte(`{"name":"seshat","subjects":[`)
	if err := os.WriteFile(path, broken, 0o644); err != nil {
		t.Fatal(err)
	}

	addToSeshatTracker(3)

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != string(broken) {
		t.Fatalf("损坏文件被改写: %q", raw)
	}
}
