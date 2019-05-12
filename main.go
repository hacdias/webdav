package main

import (
	"runtime"

	"github.com/hacdias/webdav/cmd"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	cmd.Execute()
}
