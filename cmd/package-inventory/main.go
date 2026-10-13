// Command package-inventory verifies and maintains preserved inventory evidence.
package main

import (
	"github.com/go-muse/go-musicxml/internal/packageinventory"
	"os"
)

func main() { os.Exit(packageinventory.Run(os.Args[1:], os.Stdout, os.Stderr)) }
