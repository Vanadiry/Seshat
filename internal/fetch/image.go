package fetch

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

const imagesDir = "images"

// DownloadImage 下载图片到 {dataDir}/images/{kind}/{id}.jpg，返回相对路径。
func DownloadImage(url, dataDir string, id int, kind string) (string, error) {
	if url == "" {
		return "", fmt.Errorf("empty URL")
	}
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download: HTTP %d", resp.StatusCode)
	}
	dir := filepath.Join(dataDir, imagesDir, kind)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	f, err := os.Create(filepath.Join(dir, fmt.Sprintf("%d.jpg", id)))
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/%s/%d.jpg", imagesDir, kind, id), nil
}
