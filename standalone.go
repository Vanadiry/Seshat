//go:build !ffi

package main

import (
	"fmt"
	"os"
	"time"
)

func main() {
	srv, addr := runSeshat()
	_ = srv
	time.Sleep(200 * time.Millisecond)
	if os.Getenv("SESHAT_SIDECAR") != "1" {
		openBrowser(fmt.Sprintf("http://%s", addr))
	}

	// Block forever
	select {}
}
