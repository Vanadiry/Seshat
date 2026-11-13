package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/vanadiry/seshat/core/bangumi"
	"github.com/vanadiry/seshat/core/config"
)

type upstreamSearchItem struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	NameCN   string `json:"name_cn"`
	Date     string `json:"date"`
	Platform string `json:"platform"`
}

func handleUpstreamSearch(cfg *config.Config, bg *bangumi.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cfg.Upstream.BaseURL == "" {
			writeError(w, r, http.StatusBadRequest, "未配置上游地址")
			return
		}
		var req struct {
			Keyword string `json:"keyword"`
			Limit   int    `json:"limit"`
			Offset  int    `json:"offset"`
		}
		limitBody(w, r)
		if !decodeJSON(w, r, &req) {
			return
		}
		keyword := strings.TrimSpace(req.Keyword)
		if keyword == "" {
			writeError(w, r, http.StatusBadRequest, "关键词不能为空")
			return
		}
		limit := req.Limit
		if limit <= 0 {
			limit = 20
		}
		if limit > 50 {
			limit = 50
		}
		offset := req.Offset
		if offset < 0 {
			offset = 0
		}

		path := fmt.Sprintf("v0/search/subjects?limit=%d&offset=%d", limit, offset)
		payload := map[string]any{
			"keyword": keyword,
			"sort":    "match",
			"filter":  map[string]any{"type": []int{2}},
		}
		data, err := bg.PostRaw(path, payload)
		if err != nil {
			writeError(w, r, http.StatusBadGateway, "上游搜索失败", map[string]any{
				"upstream": err.Error(),
			})
			return
		}
		items, total, err := parseUpstreamSearch(data)
		if err != nil {
			writeError(w, r, http.StatusBadGateway, "上游响应不合法", map[string]any{
				"upstream": err.Error(),
			})
			return
		}
		writeJSON(w, map[string]any{
			"total":  total,
			"limit":  limit,
			"offset": offset,
			"data":   items,
		})
	}
}

func parseUpstreamSearch(data []byte) ([]upstreamSearchItem, int, error) {
	var resp struct {
		Data  []upstreamSearchItem `json:"data"`
		Total int                  `json:"total"`
	}
	if err := json.Unmarshal(data, &resp); err == nil && resp.Data != nil {
		if resp.Total < len(resp.Data) {
			resp.Total = len(resp.Data)
		}
		return resp.Data, resp.Total, nil
	}
	var list []upstreamSearchItem
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, 0, err
	}
	return list, len(list), nil
}
