package main

import (
	"os"

	"sound-signal-generator/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:]))
}
