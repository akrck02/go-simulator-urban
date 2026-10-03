// Command go-simulator-urban is a go-simulator plugin that grows pre-industrial cities year by
// year on real relief. The host starts it; it talks over standard input and output.
package main

import (
	"log"

	"github.com/akrck02/go-simulator-urban/urban"
)

func main() {
	log.SetFlags(0)
	if err := urban.Plugin().Serve(); err != nil {
		log.Fatal(err)
	}
}
