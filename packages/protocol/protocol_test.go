package protocol

import (
	"context"
	"fmt"
	"testing"

	"github.com/iotaledger/hive.go/core/configuration"
	"github.com/iotaledger/hive.go/core/logger"
	"github.com/iotaledger/hive.go/core/serix"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/diskutil"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
)

func TestProtocol(t *testing.T) {
	require.NoError(t, logger.InitGlobalLogger(configuration.New()))

	log := logger.NewLogger(t.Name())
	baseDirectory := t.TempDir()
	diskUtil := diskutil.New(baseDirectory)

	snapshotHeaderBytes, err := serix.DefaultAPI.Encode(context.Background(), ledger.SnapshotHeader{
		OutputWithMetadataCount: 1,
		FullEpochIndex:          2,
		DiffEpochIndex:          3,
		LatestECRecord:          commitment.New(commitment.ID{4}, 5, commitment.RootsID{6}),
	})
	require.NoError(t, err)
	require.NoError(t, diskUtil.WriteFile(diskUtil.RelativePath("snapshot.bin"), snapshotHeaderBytes))

	protocol := New(nil, log, WithBaseDirectory(baseDirectory))

	fmt.Println(protocol)
}
