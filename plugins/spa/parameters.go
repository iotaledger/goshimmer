package spa

import (
	flag "github.com/spf13/pflag"
)

const (
	CFG_BIND_ADDRESS        = "dashboard.bindAddress"
	CFG_DEV                 = "dashboard.dev"
	CFG_BASIC_AUTH_ENABLED  = "dashboard.basic_auth.enabled"
	CFG_BASIC_AUTH_USERNAME = "dashboard.basic_auth.username"
	CFG_BASIC_AUTH_PASSWORD = "dashboard.basic_auth.password"
)

func init() {
	flag.String(CFG_BIND_ADDRESS, "127.0.0.1", "the bind address of the dashboard")
	flag.Bool(CFG_DEV, false, "whether the dashboard runs in dev mode")
	flag.Bool(CFG_BASIC_AUTH_ENABLED, false, "whether to enable HTTP basic auth")
	flag.String(CFG_BASIC_AUTH_USERNAME, "goshimmer", "HTTP basic auth username")
	flag.String(CFG_BASIC_AUTH_PASSWORD, "goshimmer", "HTTP basic auth password")
}
