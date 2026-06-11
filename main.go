package main

import (
	"fmt"
	stdlog "log"
	"net/http"
	"os"
	"os/exec"
	"runtime"

	"github.com/vanadiry/seshat/Core/config"
	"github.com/vanadiry/seshat/Core/log"
	"github.com/vanadiry/seshat/Core/server"
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

	dd := cfg.DataDir()
	os.MkdirAll(dd, 0o755)
	os.MkdirAll(cfg.TrackerDir(), 0o755)
	server.EnsureExcludeFile()
	lvl := cfg.Server.LogLevel
	if lvl == "" { lvl = "info" }
	log.Init(lvl)
	log.Info("Starting Seshat...")

	router := server.New(cfg, webFS)

	addr := fmt.Sprintf("%s:%d", cfg.Server.BindAddr, cfg.Server.Port)
	srv := &http.Server{Addr: addr, Handler: router}
	go func() {
		log.Info("listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			stdlog.Fatal(err)
		}
	}()

	return srv, addr
}
