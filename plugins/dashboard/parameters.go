package dashboard

import (
	flag "github.com/spf13/pflag"
)

const (
	CFG_BIND_ADDRESS = "dashboard.bindAddress"
)

func init() {
	flag.String(CFG_BIND_ADDRESS, "127.0.0.1:8081", "the bind address for the dashboard")
}
