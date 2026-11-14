package index

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vanadiry/seshat/core/store"
)

func putTestImage(t *testing.T, dd, rel string) {
	t.Helper()
	full := filepath.Join(dd, "images", rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRebuildImageIndex(t *testing.T) {
	dd := t.TempDir()
	putTestImage(t, dd, "subjects_large/1/11.jpg")
	putTestImage(t, dd, "subjects_grid/1/11.jpg")
	putTestImage(t, dd, "characters_small/2/22.jpg")
	putTestImage(t, dd, "persons_large/3/33.jpg")
	putTestImage(t, dd, "subjects_large/1/11.png")
	putTestImage(t, dd, "subjects_large/1/notanid.jpg")

	RebuildImageIndex(dd)

	subjects := readTestIndex[map[int]store.ImageEntry](t, dd, "subjects_image.json")
	if len(subjects) != 1 {
		t.Fatalf("条目图片数 = %d", len(subjects))
	}
	if subjects[11].Large != "subjects_large/1/11.jpg" || subjects[11].Grid != "subjects_grid/1/11.jpg" {
		t.Fatalf("条目图片 = %+v", subjects[11])
	}
	if subjects[11].Small != "" {
		t.Fatalf("条目小图应为空，实际 %q", subjects[11].Small)
	}

	characters := readTestIndex[map[int]store.ImageEntry](t, dd, "characters_image.json")
	if characters[22].Small != "characters_small/2/22.jpg" {
		t.Fatalf("角色图片 = %+v", characters[22])
	}

	persons := readTestIndex[map[int]store.ImageEntry](t, dd, "persons_image.json")
	if persons[33].Large != "persons_large/3/33.jpg" {
		t.Fatalf("人物图片 = %+v", persons[33])
	}
}

// TestBuildRebuildsImageIndex 确认重建索引流程包含图片索引
func TestBuildRebuildsImageIndex(t *testing.T) {
	dd := t.TempDir()
	putTestImage(t, dd, "subjects_large/1/11.jpg")

	Build(dd, nil, nil)

	if got := LoadImageIndex(dd, "subjects_image.json")[11].Large; got != "subjects_large/1/11.jpg" {
		t.Fatalf("重建后条目大图 = %q", got)
	}
}

func TestLoadImageIndexMissing(t *testing.T) {
	if got := LoadImageIndex(t.TempDir(), "subjects_image.json"); len(got) != 0 {
		t.Fatalf("缺失时应返回空映射，实际 %+v", got)
	}
}
