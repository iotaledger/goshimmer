package marker

import (
	"fmt"
	"testing"

	"github.com/iotaledger/hive.go/kvstore/mapdb"
)

func Test(t *testing.T) {
	markerSequenceManager := NewSequenceManager(mapdb.NewMapDB())

	fmt.Println(markerSequenceManager)
}
