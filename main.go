package main

import (
	"fmt"
	stdlog "log"
	"net/http"
	"os"

	"github.com/vanadiry/seshat/Core/config"
	"github.com/vanadiry/seshat/Core/log"
	"github.com/vanadiry/seshat/Core/server"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		stdlog.Fatalf("config: %v", err)
	}

	dd := cfg.DataDir()
	os.MkdirAll(dd, 0o755)
	os.MkdirAll(cfg.TrackerDir(), 0o755)
	log.Init(dd)
	log.Info("Starting Seshat...")

	router := server.New(cfg, webFS)

	addr := fmt.Sprintf("%s:%d", cfg.BindAddr, cfg.Port)
	log.Info("Listening on http://%s", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		stdlog.Fatal(err)
	}
}
