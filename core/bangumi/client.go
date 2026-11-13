package bangumi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/vanadiry/seshat/core/log"
)

var defaultTransport = &http.Transport{
	MaxIdleConns:        200,
	MaxIdleConnsPerHost: 100,
	MaxConnsPerHost:     100,
	IdleConnTimeout:     90 * time.Second,
	// 自定义 Transport 不会自动启用 HTTP/2
	// 缺少此项时 ALPN 仍协商出 h2，响应按 HTTP/1.x 解析，报 malformed HTTP response
	ForceAttemptHTTP2: true,
}

type Client struct {
	http      *http.Client
	ua        string
	baseURL   string
	tokenFunc func() string // 访问令牌，可能为空
	ctx       context.Context
}

func NewClient(ua, baseURL string, tokenFunc func() string, proxy *url.URL) *Client {
	tr := defaultTransport.Clone()
	if proxy != nil {
		tr.Proxy = http.ProxyURL(proxy)
	}
	return &Client{
		http:      &http.Client{Timeout: 30 * time.Second, Transport: tr},
		ua:        ua,
		baseURL:   baseURL,
		tokenFunc: tokenFunc,
		ctx:       context.Background(),
	}
}

// WithContext 返回绑定任务上下文的副本，取消时中断在途请求
func (c *Client) WithContext(ctx context.Context) *Client {
	cp := *c
	if ctx == nil {
		ctx = context.Background()
	}
	cp.ctx = ctx
	return &cp
}

// sleep 等待重试间隔，取消时提前返回
func (c *Client) sleep(d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-c.ctx.Done():
		return c.ctx.Err()
	}
}

// setAuth 添加认证头
func (c *Client) setAuth(req *http.Request) {
	if c.tokenFunc != nil {
		if tok := c.tokenFunc(); tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
		}
	}
}

// GetImage 下载图片，瞬时错误自动重试 3 次，404 和占位图不重试
func (c *Client) GetImage(urlPath string) ([]byte, error) {
	url := fmt.Sprintf("%s/%s", c.baseURL, urlPath)
	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		if err := c.ctx.Err(); err != nil {
			return nil, err
		}
		if attempt > 0 {
			log.Debug("retrying image request", "path", urlPath, "attempt", attempt)
			if err := c.sleep(time.Duration(attempt) * time.Second); err != nil {
				return nil, err
			}
		}
		data, err := c.doGetImage(url)
		if err == nil {
			return data, nil
		}
		lastErr = err
		if c.ctx.Err() != nil {
			return nil, c.ctx.Err()
		}
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
	req, err := http.NewRequestWithContext(c.ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
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
		if err := c.ctx.Err(); err != nil {
			return nil, err
		}
		if attempt > 0 {
			log.Debug("retrying API request", "path", urlPath, "attempt", attempt)
			if err := c.sleep(time.Duration(attempt) * time.Second); err != nil {
				return nil, err
			}
		}
		data, err := c.doGetRaw(url)
		if err == nil {
			return data, nil
		}
		lastErr = err
		if c.ctx.Err() != nil {
			return nil, c.ctx.Err()
		}
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
	req, err := http.NewRequestWithContext(c.ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", c.ua)
	req.Header.Set("Accept", "application/json")
	c.setAuth(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 512))
		if readErr != nil {
			body = []byte(fmt.Sprintf("(read body: %v)", readErr))
		}
		err := fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
		if resp.StatusCode == http.StatusUnauthorized && c.tokenFunc != nil && c.tokenFunc() != "" {
			err = fmt.Errorf("令牌已失效，任务无法继续")
		}
		return nil, err
	}
	return io.ReadAll(resp.Body)
}

// PostRaw 提交 JSON 请求，瞬时错误自动重试 3 次，404/400 不重试
func (c *Client) PostRaw(urlPath string, body any) ([]byte, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	url := fmt.Sprintf("%s/%s", c.baseURL, urlPath)
	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		if err := c.ctx.Err(); err != nil {
			return nil, err
		}
		if attempt > 0 {
			log.Debug("retrying API request", "path", urlPath, "attempt", attempt)
			if err := c.sleep(time.Duration(attempt) * time.Second); err != nil {
				return nil, err
			}
		}
		data, err := c.doPostRaw(url, payload)
		if err == nil {
			return data, nil
		}
		lastErr = err
		if c.ctx.Err() != nil {
			return nil, c.ctx.Err()
		}
		if strings.Contains(err.Error(), "HTTP 404") || strings.Contains(err.Error(), "HTTP 400") {
			return nil, err
		}
		if strings.Contains(err.Error(), "令牌") {
			return nil, err
		}
	}
	return nil, fmt.Errorf("%w (retried 3 times)", lastErr)
}

func (c *Client) doPostRaw(url string, payload []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(c.ctx, "POST", url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", c.ua)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	c.setAuth(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 512))
		if readErr != nil {
			body = []byte(fmt.Sprintf("(read body: %v)", readErr))
		}
		err := fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
		if resp.StatusCode == http.StatusUnauthorized && c.tokenFunc != nil && c.tokenFunc() != "" {
			err = fmt.Errorf("令牌已失效，任务无法继续")
		}
		return nil, err
	}
	return io.ReadAll(resp.Body)
}

// GetRawPaged 分页请求 API 端点
func (c *Client) GetRawPaged(urlPath string, offset, limit int) ([]byte, error) {
	return c.GetRaw(fmt.Sprintf("%s?limit=%d&offset=%d", urlPath, limit, offset))
}
