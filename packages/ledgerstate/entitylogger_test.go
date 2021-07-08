package ledgerstate

import (
	"fmt"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/entitylogger"

	"github.com/iotaledger/hive.go/kvstore/mapdb"
)

func Test_EntityLogger(t *testing.T) {
	entityLogger := entitylogger.New(mapdb.NewMapDB())
	entityLogger.RegisterEntity("Branch", NewBranchLogger)

	entityLogger.Logger("Branch", BranchID{1}).LogInfo("Hallo :D")
	entityLogger.Logger("Branch", BranchID{1}).LogDebug("Was")
	entityLogger.Logger("Branch", BranchID{2}).LogDebug("Was")

	time.Sleep(800 * time.Millisecond)

	fmt.Println(entityLogger.Logger("Branch").Entries())

	fmt.Println(entityLogger.Logger("Branch", BranchID{1}).Entries())
}
