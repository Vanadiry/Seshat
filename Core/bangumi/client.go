package bangumi

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/vanadiry/seshat/Core/log"
)

var defaultTransport = &http.Transport{
	MaxIdleConns:        200,
	MaxIdleConnsPerHost: 100,
	MaxConnsPerHost:     100,
	IdleConnTimeout:     90 * time.Second,
}

type Client struct {
	http      *http.Client
	ua        string
	baseURL   string
	tokenFunc func() string // 访问令牌，可能为空
}

func NewClient(ua, baseURL string, tokenFunc func() string) *Client {
	if baseURL == "" {
		baseURL = "https://api.bgm.tv"
	}
	return &Client{
		http:      &http.Client{Timeout: 30 * time.Second, Transport: defaultTransport},
		ua:        ua,
		baseURL:   baseURL,
		tokenFunc: tokenFunc,
	}
}

// setAuth 添加认证头。
func (c *Client) setAuth(req *http.Request) {
	if c.tokenFunc != nil {
		if tok := c.tokenFunc(); tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
		}
	}
}

// GetImage 下载图片，瞬时错误自动重试 3 次，404 和占位图不重试。
func (c *Client) GetImage(urlPath string) ([]byte, error) {
	url := fmt.Sprintf("%s/%s", c.baseURL, urlPath)
	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		if attempt > 0 {
			log.Debug("retrying image request", "path", urlPath, "attempt", attempt)
			time.Sleep(time.Duration(attempt) * time.Second)
		}
		data, err := c.doGetImage(url)
		if err == nil {
			return data, nil
		}
		lastErr = err
		// 404 和占位图不重试
		if strings.Contains(err.Error(), "HTTP 404") || strings.Contains(err.Error(), "placeholder") {
			return nil, err
		}
	}
	return nil, fmt.Errorf("%w (retried 3 times)", lastErr)
}

func (c *Client) doGetImage(url string) ([]byte, error) {
	client := &http.Client{
		Timeout:   30 * time.Second,
		Transport: c.http.Transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if strings.Contains(req.URL.String(), "no_icon") {
				return fmt.Errorf("placeholder")
			}
			return nil
		},
	}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", c.ua)
	c.setAuth(req)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// GetRaw 请求 API 端点，瞬时错误自动重试 3 次，404/400 不重试
func (c *Client) GetRaw(urlPath string) ([]byte, error) {
	url := fmt.Sprintf("%s/%s", c.baseURL, urlPath)
	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		if attempt > 0 {
			log.Debug("retrying API request", "path", urlPath, "attempt", attempt)
			time.Sleep(time.Duration(attempt) * time.Second)
		}
		data, err := c.doGetRaw(url)
		if err == nil {
			return data, nil
		}
		lastErr = err
		// 4xx 错误不重试
		if strings.Contains(err.Error(), "HTTP 404") || strings.Contains(err.Error(), "HTTP 400") {
			return nil, err
		}
		if strings.Contains(err.Error(), "令牌") {
			return nil, err
		}
	}
	return nil, fmt.Errorf("%w (retried 3 times)", lastErr)
}

func (c *Client) doGetRaw(url string) ([]byte, error) {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", c.ua)
	req.Header.Set("Accept", "application/json")
	c.setAuth(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		err := fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
		if resp.StatusCode == http.StatusUnauthorized && c.tokenFunc != nil && c.tokenFunc() != "" {
			err = fmt.Errorf("你设置了令牌，但已失效。请终止任务并退出程序，检查令牌后重试。")
		}
		return nil, err
	}
	return io.ReadAll(resp.Body)
}

// GetRawPaged 分页请求 API 端点
func (c *Client) GetRawPaged(urlPath string, offset, limit int) ([]byte, error) {
	return c.GetRaw(fmt.Sprintf("%s?limit=%d&offset=%d", urlPath, limit, offset))
}
