package webauth

import (
	flag "github.com/spf13/pflag"
)

const (
	UI_USER   = "ui.user"
	UI_PASS   = "ui.pass"
	UI_JWTKEY = "ui.jwtkey"
)

func init() {
	flag.String(UI_USER, "user", "username for web ui")
	flag.String(UI_PASS, "pass", "password for web ui")
	flag.String(UI_JWTKEY, "", "key for jwt signing")
}
