package retainer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/iotaledger/hive.go/core/serix"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/models"
)

func TestRetainer_BlockMetadata_Serialization(t *testing.T) {
	var blockID1, blockID2 models.BlockID
	_ = blockID1.FromRandomness()
	_ = blockID2.FromRandomness()

	meta := newBlockMetadata(nil)
	meta.M.Missing = false
	meta.M.Solid = true
	meta.M.Invalid = false
	meta.M.Orphaned = true
	meta.M.OrphanedBlocksInPastCone = make(models.BlockIDs)
	//meta.M.OrphanedBlocksInPastCone.Add(blockID1)
	//meta.M.OrphanedBlocksInPastCone.Add(blockID2)

	fmt.Println(meta.Encode())
}

func TestRetainer_BlockMetadata_JSON(t *testing.T) {
	var blockID1, blockID2 models.BlockID
	_ = blockID1.FromRandomness()
	_ = blockID2.FromRandomness()

	meta := newBlockMetadata(nil)
	meta.M.Missing = false
	meta.M.Solid = true
	meta.M.Invalid = false
	meta.M.Orphaned = true
	meta.M.OrphanedBlocksInPastCone = make(models.BlockIDs)
	meta.M.OrphanedBlocksInPastCone.Add(blockID1)

	out, err := serix.DefaultAPI.JSONEncode(context.Background(), meta.M)
	require.NoError(t, err)
	printPrettyJSON(t, out)
}

func printPrettyJSON(t *testing.T, b []byte) {
	var prettyJSON bytes.Buffer
	require.NoError(t, json.Indent(&prettyJSON, b, "", "    "))
	fmt.Println(prettyJSON.String())
}
