package messagelayer

import flag "github.com/spf13/pflag"

func init() {
	flag.String(CfgMessageLayerSnapshotFile, "./snapshot.bin", "the path to the snapshot file")
	flag.Int(CfgMessageLayerFCOBAverageNetworkDelay, 5, "the avg. network delay to use for FCoB rules")
	flag.Int(CfgTangleWidth, 0, "the width of the Tangle")
}
