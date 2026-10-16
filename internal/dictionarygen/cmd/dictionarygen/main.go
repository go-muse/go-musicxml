// Command dictionarygen prepares committed runtime assets from pinned local
// inputs. It deliberately has no download or latest-version mode.
package main

import (
	"flag"
	"log"

	"github.com/go-muse/go-musicxml/internal/dictionarygen"
)

func main() {
	root := flag.String("root", ".", "repository root containing committed dictionary inputs")
	flag.Parse()
	if flag.NArg() != 0 {
		log.Fatal("dictionarygen takes only the optional -root flag")
	}
	if err := dictionarygen.Generate(*root); err != nil {
		log.Fatal(err)
	}
}
