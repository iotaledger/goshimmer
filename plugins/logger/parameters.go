package logger

import (
	flag "github.com/spf13/pflag"
)

const (
	// CfgLoggerLevel defines the logger's level.
	CfgLoggerLevel = "logger.level"
	// CfgLoggerDisableCaller defines whether to disable caller info.
	CfgLoggerDisableCaller = "logger.disableCaller"
	// CfgLoggerDisableStacktrace defines whether to disable stack trace info.
	CfgLoggerDisableStacktrace = "logger.disableStacktrace"
	// CfgLoggerEncoding defines the logger's encoding.
	CfgLoggerEncoding = "logger.encoding"
	// CfgLoggerOutputPaths defines the logger's output paths.
	CfgLoggerOutputPaths = "logger.outputPaths"
	// CfgLoggerDisableEvents defines whether to disable logger events.
	CfgLoggerDisableEvents = "logger.disableEvents"
)

func initFlags() {
	flag.String(CfgLoggerLevel, "info", "log level")
	flag.Bool(CfgLoggerDisableCaller, false, "disable caller info in log")
	flag.Bool(CfgLoggerDisableStacktrace, false, "disable stack trace in log")
	flag.String(CfgLoggerEncoding, "console", "log encoding")
	flag.StringSlice(CfgLoggerOutputPaths, []string{"stdout", "goshimmer.log"}, "log output paths")
	flag.Bool(CfgLoggerDisableEvents, true, "disable logger events")
}
