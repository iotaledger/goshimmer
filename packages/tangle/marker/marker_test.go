package marker

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

func Test(t *testing.T) {
	branch1 := ledgerstate.BranchID{2}
	branch2 := ledgerstate.BranchID{3}

	b := &RangeMap{}
	b.AddMapping(2, ledgerstate.UndefinedBranchID)
	b.AddMapping(10, branch1)
	b.AddMapping(17, branch2)

	branchID, err := b.MappedValue(11)
	assert.NoError(t, err)
	assert.Equal(t, branch1, branchID)

	branchID, err = b.MappedValue(17)
	assert.NoError(t, err)
	assert.Equal(t, branch2, branchID)

	branchID, err = b.MappedValue(10)
	assert.NoError(t, err)
	assert.Equal(t, branch1, branchID)

	branchID, err = b.MappedValue(1)
	assert.Error(t, err)

	fmt.Println(b)
}
