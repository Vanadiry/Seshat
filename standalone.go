//go:build !ffi

package main

import (
	"fmt"
	"time"
)

func main() {
	srv, addr := runSeshat()
	_ = srv
	time.Sleep(200 * time.Millisecond)
	openBrowser(fmt.Sprintf("http://%s", addr))

	// Block forever
	select {}
}
