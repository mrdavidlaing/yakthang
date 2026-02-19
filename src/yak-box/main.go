// Package main is the entry point for yak-box CLI application.
package main

import (
	"log"

	"github.com/yakthang/yakbox/cmd"
)

var version = "dev"

func main() {
	cmd.SetVersion(version)
	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
