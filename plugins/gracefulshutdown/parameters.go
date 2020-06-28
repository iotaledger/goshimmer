package gracefulshutdown

import (
	flag "github.com/spf13/pflag"
)

const (
	// CfgWaitToKillTimeInSeconds the maximum amount of time to wait for background processes to terminate.
	CfgWaitToKillTimeInSeconds = "gracefulshutdown.waitToKillTime"
)

func init() {
	flag.Int(CfgWaitToKillTimeInSeconds, 60, "the maximum amount of time to wait for background processes to terminate, in seconds")
}
