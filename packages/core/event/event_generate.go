//go:build ignore

package main

import (
	"os"
	"strconv"

	"github.com/iotaledger/goshimmer/packages/core/generate"
)

func main() {
	if len(os.Args) != 2 {
		panic("expected one argument")
	}

	paramCount, err := strconv.Atoi(os.Args[1])
	if err != nil {
		panic(err)
	}

	generate.VariadicGeneric(os.Getenv("GOFILE"), "event.go", paramCount)
}
