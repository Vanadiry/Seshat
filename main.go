package main

import (
	"fmt"
	stdlog "log"
	"net/http"
	"os"

	"github.com/vanadiry/seshat/internal/bangumi"
	"github.com/vanadiry/seshat/internal/config"
	"github.com/vanadiry/seshat/internal/db"
	"github.com/vanadiry/seshat/internal/fetch"
	"github.com/vanadiry/seshat/internal/handler"
	"github.com/vanadiry/seshat/internal/log"
	"github.com/vanadiry/seshat/internal/query"
	"github.com/vanadiry/seshat/internal/server"
	"github.com/vanadiry/seshat/internal/task"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		stdlog.Fatalf("config: %v", err)
	}

	dataDir := cfg.DataDir()
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		stdlog.Fatalf("create data dir %s: %v", dataDir, err)
	}

	log.Init(dataDir)
	log.Info("Starting Seshat...")

	database, err := db.Open(fmt.Sprintf("%s/seshat.db", dataDir))
	if err != nil {
		stdlog.Fatalf("main db: %v", err)
	}
	defer database.Close()
	log.Info("Database connected")

	bgClient := bangumi.NewClient("Seshat/Test")

	fetchSvc := &fetch.Service{
		Client:      bgClient,
		DB:          database,
		DataDir:     dataDir,
		Concurrency: cfg.Concurrency,
		Tasks:       task.NewManager(),
	}

	h := &handler.Handler{
		Queries: query.New(database),
		Config:  cfg,
		Fetch:   fetchSvc,
		DataDir: dataDir,
	}
	router := server.New(h, webFS)

	addr := fmt.Sprintf("%s:%d", cfg.BindAddr, cfg.Port)
	log.Info("Listening on http://%s (data: %s)", addr, dataDir)
	if err := http.ListenAndServe(addr, router); err != nil {
		stdlog.Fatal(err)
	}
}
