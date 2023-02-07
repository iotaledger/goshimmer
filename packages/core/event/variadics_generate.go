//go:build ignore

package main

import (
	"os"
	"strconv"

	"github.com/iotaledger/goshimmer/packages/core/generate"
)

// This file is used to generate the variadic generic event implementations.
func main() {
	if len(os.Args) != 2 {
		panic("expected one argument")
	}

	paramCount, paramCountErr := strconv.Atoi(os.Args[1])
	if paramCountErr != nil {
		panic(paramCountErr)
	}

	template, templateErr := generate.NewVariadicGenericsTemplate(os.Getenv("GOFILE"))
	if templateErr != nil {
		panic(templateErr)
	}

	if generateErr := template.Generate("variadics.go", paramCount); generateErr != nil {
		panic(generateErr)
	}
}
