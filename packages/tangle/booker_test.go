package tangle

import (
	"fmt"
	"testing"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/stretchr/testify/require"
)

func TestMarkerIndexBranchMapping_String(t *testing.T) {
	mapping := NewMarkerIndexBranchIDMapping(1337)
	mapping.SetBranchID(4, ledgerstate.UndefinedBranchID)
	mapping.SetBranchID(24, ledgerstate.MasterBranchID)

	fmt.Println(mapping)

	mappingClone, _, err := MarkerIndexBranchIDMappingFromBytes(mapping.Bytes())
	require.NoError(t, err)

	fmt.Println(mappingClone)
}
