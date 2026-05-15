package main

import (
	"fmt"
	stdlog "log"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"

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

func main() {
	cfg, err := config.Load()
	if err != nil {
		stdlog.Fatalf("config: %v", err)
	}

	dd := cfg.DataDir()
	os.MkdirAll(dd, 0o755)
	os.MkdirAll(cfg.TrackerDir(), 0o755)
	log.Init()
	log.Info("Starting Seshat...")

	router := server.New(cfg, webFS)

	addr := fmt.Sprintf("%s:%d", cfg.Server.BindAddr, cfg.Server.Port)
	log.Info("Listening on http://%s", addr)

	go func() {
		time.Sleep(200 * time.Millisecond)
		openBrowser(fmt.Sprintf("http://%s", addr))
	}()

	if err := http.ListenAndServe(addr, router); err != nil {
		stdlog.Fatal(err)
	}
}
