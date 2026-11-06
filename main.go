package main

import (
	"os"

	"github.com/777pxy/incidentio-takehome/cmd"
)

func main() {
	os.Exit(cmd.Run(os.Args[1:], os.Stdout, os.Stderr))
}
