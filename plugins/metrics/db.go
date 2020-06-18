package metrics

import (
	"os"
	"path/filepath"

	"github.com/iotaledger/goshimmer/packages/metrics"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/goshimmer/plugins/database"
	"github.com/iotaledger/hive.go/events"
	"go.uber.org/atomic"
)

var (
	dbSize atomic.Uint64

	onDBSize = events.NewClosure(func(size uint64) {
		dbSize.Store(size)
	})
)

// DBSize returns the size of the db.
func DBSize() uint64 {
	return dbSize.Load()
}

func colectDBSize() {
	dbSize, err := directorySize(config.Node.GetString(database.CfgDatabaseDir))
	if err == nil {
		metrics.Events().DBSize.Trigger(uint64(dbSize))
	}
}

func directorySize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return err
	})
	return size, err
}
