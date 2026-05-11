package bangumi

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	http    *http.Client
	ua      string
	baseURL string
}

func NewClient(ua, baseURL string) *Client {
	if baseURL == "" {
		baseURL = "https://api.bgm.tv"
	}
	return &Client{http: &http.Client{Timeout: 30 * time.Second}, ua: ua, baseURL: baseURL}
}


// GetImage downloads the actual image binary from an official image endpoint.
// The Bangumi API returns a 302 redirect; Go's http.Client follows it automatically.
func (c *Client) GetImage(urlPath string) ([]byte, error) {
	url := fmt.Sprintf("%s/%s", c.baseURL, urlPath)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", c.ua)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// GetRaw fetches a raw API path and returns the response body.
func (c *Client) GetRaw(urlPath string) ([]byte, error) {
	url := fmt.Sprintf("%s/%s", c.baseURL, urlPath)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", c.ua)
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return io.ReadAll(resp.Body)
}

// GetRawPaged fetches a paginated URL with offset and limit.
func (c *Client) GetRawPaged(urlPath string, offset, limit int) ([]byte, error) {
	return c.GetRaw(fmt.Sprintf("%s?limit=%d&offset=%d", urlPath, limit, offset))
}
