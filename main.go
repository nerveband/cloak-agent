package main

import (
	"os"

	"github.com/nerveband/cloak-agent/cmd"
)

func main() {
	if err := cmd.Execute(os.Args[1:]); err != nil {
		os.Exit(cmd.PrintCLIError(err, os.Args[1:]))
	}
}
