package entitylogger

import (
	"fmt"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/kvstore/mapdb"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

func Test_EntityLogger(t *testing.T) {
	entityLogger := New(mapdb.NewMapDB())
	entityLogger.RegisterEntity("Branch", NewBranchLogger)

	entityLogger.Logger("Branch", ledgerstate.BranchID{1}).LogInfo("Hallo :D")
	entityLogger.Logger("Branch", ledgerstate.BranchID{1}).LogDebug("Was")
	entityLogger.Logger("Branch", ledgerstate.BranchID{2}).LogDebug("Was")

	time.Sleep(800 * time.Millisecond)

	fmt.Println(entityLogger.Logger("Branch").Entries())

	fmt.Println(entityLogger.Logger("Branch", ledgerstate.BranchID{1}).Entries())
}
