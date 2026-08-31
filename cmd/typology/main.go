// Command typology discovers, validates, and emits architecture catalogs.
package main

import (
	"os"

	"github.com/behaviorengineering/typology/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
