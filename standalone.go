//go:build !ffi

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/vanadiry/seshat/core/server"
)

func main() {
	srv, addr := runSeshat()
	time.Sleep(200 * time.Millisecond)
	if os.Getenv("SESHAT_SIDECAR") != "1" {
		openBrowser(fmt.Sprintf("http://%s", addr))
	}

	// 等待退出信号，取消任务后关闭服务
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig

	server.CancelAllTasks()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}
