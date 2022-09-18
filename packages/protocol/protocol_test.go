package protocol

import (
	"context"
	"fmt"
	"testing"

	"github.com/iotaledger/hive.go/core/serix"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/diskutil"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
)

func TestProtocol(t *testing.T) {
	baseDirectory := t.TempDir()
	diskUtil := diskutil.New(baseDirectory)

	snapshotCommitment := commitment.New(commitment.ID{1}, 2, commitment.RootsID{3})

	snapshotHeader := ledger.SnapshotHeader{
		OutputWithMetadataCount: 1,
		FullEpochIndex:          2,
		DiffEpochIndex:          3,
		LatestECRecord:          snapshotCommitment,
	}

	snapshotHeaderBytes, err := serix.DefaultAPI.Encode(context.Background(), &snapshotHeader)
	require.NoError(t, err)
	fmt.Println(snapshotHeaderBytes)

	require.NoError(t, diskUtil.WriteFile(diskUtil.RelativePath("snapshot.bin"), snapshotHeaderBytes))

	protocol := New(nil, nil, WithBaseDirectory(baseDirectory))

	fmt.Println(protocol)
}
