package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func AddBoolParameter(p *bool, name string, usage string) {
	flag.BoolVar(p, name, *p, usage)
}

func AddIntParameter(p *int, name string, usage string) {
	flag.IntVar(p, name, *p, usage)
}

func AddStringParameter(p *string, name string, usage string) {
	flag.StringVar(p, name, *p, usage)
}

func printUsage() {
	_, err := fmt.Fprintf(
		os.Stderr,
		"\n"+
			"SHIMMER 1.0\n\n"+
			"  A lightweight modular IOTA node.\n\n"+
			"Usage:\n\n"+
			"  %s [OPTIONS]\n\n"+
			"Options:\n\n",
		filepath.Base(os.Args[0]),
	)
	if err != nil {
		panic(err)
	}

	flag.PrintDefaults()
}
