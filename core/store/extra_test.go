package store

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestListAndListIDs(t *testing.T) {
	dd := t.TempDir()
	for _, id := range []int{11, 22} {
		if err := Put(dd, Key("subjects", id, "info.json"), []byte("{}")); err != nil {
			t.Fatal(err)
		}
	}

	names, err := List(dd, "subjects")
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 2 {
		t.Fatalf("List = %v", names)
	}
	ids, err := ListIDs(dd, "subjects")
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 || ids[0] != 11 || ids[1] != 22 {
		t.Fatalf("ListIDs = %v", ids)
	}

	if _, err := List(dd, "missing"); err == nil {
		t.Fatal("不存在的领域应返回错误")
	}
}

func TestStripImages(t *testing.T) {
	input := []byte(`{
		"id": 1,
		"images": {"large": "a.jpg"},
		"characters": [
			{"id": 2, "image": "b.jpg", "name": "n"},
			{"id": 3, "images": {"grid": "c.jpg"}, "name": "m"}
		]
	}`)
	out := StripImages(input)
	if bytes.Contains(out, []byte(`"images"`)) || bytes.Contains(out, []byte(`"image"`)) {
		t.Fatalf("images 字段没有清理: %s", out)
	}

	var v map[string]any
	if err := json.Unmarshal(out, &v); err != nil {
		t.Fatal(err)
	}
	if v["id"] != float64(1) {
		t.Fatalf("其他字段被破坏: %v", v)
	}
	chars := v["characters"].([]any)
	if chars[0].(map[string]any)["name"] != "n" {
		t.Fatalf("嵌套字段被破坏: %v", chars[0])
	}

	invalid := []byte("not json")
	if !bytes.Equal(StripImages(invalid), invalid) {
		t.Fatal("不合法 JSON 应原样返回")
	}
}
