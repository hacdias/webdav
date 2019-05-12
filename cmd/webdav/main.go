package main

import (
	"log"
	"runtime"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
