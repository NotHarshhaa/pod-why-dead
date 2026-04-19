package main

import (
	"os"

	"github.com/NotHarshhaa/pod-why-dead/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
