package main

import (
	"os"

	"github.com/flowkater/ax/cmd/ax"
)

func main() {
	if err := ax.NewRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
