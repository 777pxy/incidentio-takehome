package main

import (
	"os"

	"github.com/777pxy/incidentio-takehome/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
