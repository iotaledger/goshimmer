package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/iotaledger/goshimmer/packages/node"
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

var enabledPlugins []string
var disabledPlugins []string

func AddPluginStatus(name string, status int) {
	switch status {
	case node.Enabled:
		enabledPlugins = append(enabledPlugins, name)
	case node.Disabled:
		disabledPlugins = append(disabledPlugins, name)
	}
}

func getList(a []string) string {
	sort.Strings(a)
	return strings.Join(a, " ")
}

func printUsage() {
	fmt.Fprintf(
		os.Stderr,
		"\n"+
			"SHIMMER\n\n"+
			"  A lightweight modular IOTA node.\n\n"+
			"Usage:\n\n"+
			"  %s [OPTIONS]\n\n"+
			"Options:\n",
		filepath.Base(os.Args[0]),
	)
	flag.PrintDefaults()

	fmt.Fprintf(os.Stderr, "\nThe following plugins are enabled by default and can be disabled with -%s:\n  %s\n", getFlagName(node.DISABLE_PLUGINS.Name), getList(enabledPlugins))
	fmt.Fprintf(os.Stderr, "The following plugins are disabled by default and can be enabled with -%s:\n  %s\n\n", getFlagName(node.ENABLE_PLUGINS.Name), getList(disabledPlugins))
}
