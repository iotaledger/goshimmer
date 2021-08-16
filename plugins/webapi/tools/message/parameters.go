package message

import flag "github.com/spf13/pflag"

const (
	// CfgExportPath the directory where exported files sit.
	CfgExportPath = "webapi.exportPath"
)

func init() {
	flag.String(CfgExportPath, ".", "default export path")
}
