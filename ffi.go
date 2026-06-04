//go:build cgo

package main

/*
#include <stdlib.h>
*/
import "C"
import (
	"net/http"
	"sync"
)

var ffiServer *http.Server
var ffiMu sync.Mutex

//export StartSeshat
func StartSeshat() C.int {
	ffiMu.Lock()
	defer ffiMu.Unlock()
	if ffiServer != nil {
		return -1
	}
	srv, _ := runSeshat()
	ffiServer = srv
	return 0
}

//export StopSeshat
func StopSeshat() {
	ffiMu.Lock()
	defer ffiMu.Unlock()
	if ffiServer != nil {
		ffiServer.Close()
		ffiServer = nil
	}
}

func main() {}
