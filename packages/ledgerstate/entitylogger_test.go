package ledgerstate

import (
	"fmt"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/entitylog"

	"github.com/iotaledger/hive.go/kvstore/mapdb"
)

func Test_EntityLogger(t *testing.T) {
	entityLog := entitylog.New(mapdb.NewMapDB())
	entityLog.RegisterEntity(BranchEntityName, NewBranchLogger)

	branch1Logger := entityLog.Logger(BranchEntityName, BranchID{1})
	branch1Logger.LogInfo("Hallo :D")
	branch1Logger.LogDebug("Was")

	entityLog.Logger(BranchEntityName, BranchID{2}).LogDebug("Was")

	time.Sleep(800 * time.Millisecond)

	fmt.Println(entityLog.Logger(BranchEntityName).Entries())

	fmt.Println(branch1Logger.Entries())
}
