package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/vanadiry/seshat/internal/bangumi"
	"github.com/vanadiry/seshat/internal/config"
	"github.com/vanadiry/seshat/internal/db"
	"github.com/vanadiry/seshat/internal/fetch"
	"github.com/vanadiry/seshat/internal/handler"
	"github.com/vanadiry/seshat/internal/query"
	"github.com/vanadiry/seshat/internal/server"
	"github.com/vanadiry/seshat/internal/task"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	dataDir := cfg.DataDir()
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		log.Fatalf("create data dir %s: %v", dataDir, err)
	}

	database, err := db.Open(fmt.Sprintf("%s/seshat.db", dataDir))
	if err != nil {
		log.Fatalf("main db: %v", err)
	}
	defer database.Close()

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
	}
	router := server.New(h)

	addr := fmt.Sprintf("%s:%d", cfg.BindAddr, cfg.Port)
	log.Printf("seshatd 已启动，监听 http://%s (数据目录: %s)", addr, dataDir)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatal(err)
	}
}
