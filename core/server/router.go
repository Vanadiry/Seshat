package server

import (
	"net/http"
	"sort"
)

type routeKind int

const (
	kindPage routeKind = iota
	kindCore
	kindFeature
	kindLocal
)

type routeInfo struct {
	kind    routeKind
	feature string
}

type router struct {
	mux    *http.ServeMux
	routes map[string]routeInfo
}

func newRouter() *router {
	return &router{
		mux:    http.NewServeMux(),
		routes: make(map[string]routeInfo),
	}
}

func (r *router) page(pattern string, h http.HandlerFunc) {
	r.register(kindPage, "", pattern, h)
}

func (r *router) core(pattern string, h http.HandlerFunc) {
	r.register(kindCore, "", pattern, h)
}

func (r *router) feature(name, pattern string, h http.HandlerFunc) {
	if name == "" {
		panic("feature 名不能为空")
	}
	r.register(kindFeature, name, pattern, h)
}

func (r *router) local(pattern string, h http.HandlerFunc) {
	r.register(kindLocal, "", pattern, h)
}

func (r *router) register(kind routeKind, feature, pattern string, h http.HandlerFunc) {
	if _, exists := r.routes[pattern]; exists {
		panic("路由重复注册: " + pattern)
	}
	r.mux.HandleFunc(pattern, h)
	r.routes[pattern] = routeInfo{kind: kind, feature: feature}
}

func (r *router) features() []string {
	seen := make(map[string]struct{})
	names := make([]string, 0, len(r.routes))
	for _, info := range r.routes {
		if info.kind != kindFeature {
			continue
		}
		if _, ok := seen[info.feature]; ok {
			continue
		}
		seen[info.feature] = struct{}{}
		names = append(names, info.feature)
	}
	sort.Strings(names)
	return names
}
