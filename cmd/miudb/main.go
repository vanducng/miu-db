package main

import (
	"os"

	"github.com/vanducng/miu-db/internal/cli"
)

func main() {
	if err := cli.Execute(os.Args[1:]); err != nil {
		os.Exit(cli.ExitCode(err))
	}
}
