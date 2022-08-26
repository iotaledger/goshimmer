package retainer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/core/serix"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/tangle/models"
)

func TestRetainer_BlockMetadata_Serialization(t *testing.T) {
	var blockID1, blockID2 models.BlockID
	_ = blockID1.FromRandomness()
	_ = blockID2.FromRandomness()

	metadata := &BlockMetadata{
		Missing:  false,
		Solid:    false,
		Invalid:  false,
		Orphaned: false,
		// OrphanedBlocksInPastCone: models.NewBlockIDs(blockID1, blockID2),
		// StrongChildren:           nil,
		// WeakChildren:             nil,
		// LikedInsteadChildren:     nil,
		SolidTime: time.Now(),
	}

	fmt.Println(serix.DefaultAPI.Encode(context.Background(), metadata))
}

func TestRetainer_BlockMetadata_JSON(t *testing.T) {
	var blockID1, blockID2 models.BlockID
	_ = blockID1.FromRandomness()
	_ = blockID2.FromRandomness()

	metadata := &BlockMetadata{
		Missing:                  false,
		Solid:                    false,
		Invalid:                  false,
		Orphaned:                 false,
		OrphanedBlocksInPastCone: []string{blockID1.Base58(), blockID2.Base58()},
		// StrongChildren:           nil,
		// WeakChildren:             nil,
		// LikedInsteadChildren:     nil,
		SolidTime: time.Now(),
	}

	api := serix.NewAPI()

	out, err := api.JSONEncode(context.Background(), metadata)
	require.NoError(t, err)
	printPrettyJSON(t, out)
}

func printPrettyJSON(t *testing.T, b []byte) {
	var prettyJSON bytes.Buffer
	require.NoError(t, json.Indent(&prettyJSON, b, "", "    "))
	fmt.Println(prettyJSON.String())
}
