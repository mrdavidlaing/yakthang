// Package main is the entry point for yak-box CLI application.
package main

import (
	"log"

	"github.com/mrdavidlaing/yakthang/src/yak-box/cmd"
)

var version = "dev"

func main() {
	cmd.SetVersion(version)
	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
