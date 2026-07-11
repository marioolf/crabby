// Command crabby is the Linux (WSL) entrypoint.
package main

import (
	"os"

	"github.com/marioolf/crabby/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
