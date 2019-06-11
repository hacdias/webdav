package main

import (
	"runtime"

	"github.com/hacdias/webdav/v2/cmd"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	cmd.Execute()
}
