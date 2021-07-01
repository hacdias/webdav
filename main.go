package main

import (
	"context"
	"github.com/hacdias/webdav/v3/lib"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/hacdias/webdav/v3/cmd"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	ctx, cancel := context.WithCancel(context.Background())

	go lib.LastRequestLogIndex(ctx)
	go cmd.Execute()

	sigterm := make(chan os.Signal, 1)
	signal.Notify(sigterm, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT)

	<-sigterm
	log.Println("receive stop signal")

	cancel()
	// wait for other goroutines to quit
	time.Sleep(2 * time.Second)
}
