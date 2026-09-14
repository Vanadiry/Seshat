package main

import (
	"fmt"
	stdlog "log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"

	"github.com/vanadiry/seshat/core/config"
	"github.com/vanadiry/seshat/core/events"
	"github.com/vanadiry/seshat/core/log"
	"github.com/vanadiry/seshat/core/server"
)

func openBrowser(url string) {
	var cmd string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
	case "linux":
		cmd = "xdg-open"
	case "windows":
		cmd = "cmd"
	default:
		return
	}
	args := []string{url}
	if cmd == "cmd" {
		args = []string{"/c", "start", url}
	}
	exec.Command(cmd, args...).Start()
}

// runSeshat initializes the server and starts listening.
// Returns the http.Server for graceful shutdown.
func runSeshat() (*http.Server, string) {
	cfg, err := config.Load()
	if err != nil {
		stdlog.Fatalf("config: %v", err)
	}
	if errs := cfg.Validate(); len(errs) > 0 {
		// SESHAT_ERROR=<标题> 指定错误页标题，其余行作为正文
		fmt.Fprintln(os.Stderr, "SESHAT_ERROR=配置有误")
		for _, e := range errs {
			fmt.Fprintln(os.Stderr, e)
		}
		os.Exit(1)
	}

	dd := cfg.DataDir()
	os.MkdirAll(dd, 0o755)
	os.MkdirAll(cfg.TrackerDir(), 0o755)
	server.EnsureExcludeFile()
	log.Init(cfg.Server.LogLevel)
	log.Info("Starting Seshat...")
	events.InitBus()
	for _, w := range cfg.Warnings() {
		events.Bus.Warn(w)
	}

	router := server.New(cfg, webFS)

	addr := fmt.Sprintf("%s:%d", cfg.Server.BindAddr, cfg.Server.Port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		stdlog.Fatalf("listen %s: %v", addr, err)
	}
	actual := ln.Addr().String()

	// 向Tauri报告实际监听地址
	fmt.Fprintf(os.Stdout, "SESHAT_ADDR=%s\n", actual)

	srv := &http.Server{Addr: actual, Handler: router}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				stdlog.Printf("server goroutine panic: %v", r)
			}
		}()
		log.Info("listening", "addr", actual)
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			stdlog.Fatal(err)
		}
	}()

	return srv, actual
}
