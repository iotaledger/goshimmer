package snapshot

import (
	"os"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/core/crypto/ed25519"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/types/confirmation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/ledger"
	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/core/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/core/tangleold"
)

const (
	cfgPledgeTokenAmount = 1000000000000000
	snapshotFileName     = "tmp.snapshot"
)

var nodesToPledge = []string{
	"2GtxMQD94KvDH1SJPJV7icxofkyV1njuUZKtsqKmtux5", // peer_master
	"AXSoTPcN6SNwH64tywpz4k2XfAc24NR7ckKX8wPjeUZD", // peer_master2
	"FZ6xmPZXRs2M8z9m9ETTQok4PCga4X8FRHwQE6uYm4rV", // faucet
}

var (
	outputsWithMetadata = make([]*ledger.OutputWithMetadata, 0)
	epochDiffs          = make([]*ledger.EpochDiff, 0)
	manaDistribution    = createManaDistribution(cfgPledgeTokenAmount)
	solidEntryPoints    = make([]*SolidEntryPoints, 0)
)

func Test_CreateAndReadSnapshot(t *testing.T) {
	header := createSnapshot(t)

	rheader, rseps, rstates, repochDiffs := readSnapshot(t)
	compareSnapshotHeader(t, header, rheader)
	compareOutputWithMetadataSlice(t, outputsWithMetadata, rstates)
	compareEpochDiffs(t, epochDiffs, repochDiffs)
	compareSolidEntryPoints(t, solidEntryPoints, rseps)

	err := os.Remove(snapshotFileName)
	require.NoError(t, err)
}

func Test_CreateAndReadEmptySnapshot(t *testing.T) {
	// clear all data
	outputsWithMetadata = make([]*ledger.OutputWithMetadata, 0)
	epochDiffs = make([]*ledger.EpochDiff, 0)
	manaDistribution = createManaDistribution(cfgPledgeTokenAmount)
	solidEntryPoints = make([]*SolidEntryPoints, 0)

	header := createEmptySnapshot(t)

	rheader, rseps, rstates, repochDiffs := readSnapshot(t)
	compareSnapshotHeader(t, header, rheader)
	compareOutputWithMetadataSlice(t, outputsWithMetadata, rstates)
	compareEpochDiffs(t, epochDiffs, repochDiffs)
	compareSolidEntryPoints(t, solidEntryPoints, rseps)

	err := os.Remove(snapshotFileName)
	require.NoError(t, err)
}

func createEmptySnapshot(t *testing.T) (header *ledger.SnapshotHeader) {
	fullEpochIndex := epoch.Index(0)
	diffEpochIndex := epoch.Index(0)

	headerProd := func() (header *ledger.SnapshotHeader, err error) {
		ecRecord := epoch.NewECRecord(diffEpochIndex)
		ecRecord.SetECR(epoch.MerkleRoot{})
		ecRecord.SetPrevEC(epoch.MerkleRoot{})

		header = &ledger.SnapshotHeader{
			FullEpochIndex: fullEpochIndex,
			DiffEpochIndex: diffEpochIndex,
			LatestECRecord: ecRecord,
		}

		return
	}

	// prepare outputsWithMetadata
	utxoStatesProd := func() *ledger.OutputWithMetadata {
		return nil
	}

	epochDiffsProd := func() (diffs *ledger.EpochDiff) {
		outputs := make([]*ledger.OutputWithMetadata, 0)
		diffs = ledger.NewEpochDiff(outputs, outputs)
		return
	}

	seps := &SolidEntryPoints{EI: 0, Seps: make([]tangleold.BlockID, 0)}
	solidEntryPoints = append(solidEntryPoints, seps)
	sepsProd := func() (s *SolidEntryPoints) {
		return seps
	}

	header, err := CreateSnapshot(snapshotFileName, headerProd, sepsProd, utxoStatesProd, epochDiffsProd)
	require.NoError(t, err)

	return header
}

func createSnapshot(t *testing.T) (header *ledger.SnapshotHeader) {
	fullEpochIndex := epoch.Index(1)
	diffEpochIndex := epoch.Index(3)

	headerProd := func() (header *ledger.SnapshotHeader, err error) {
		ecRecord := epoch.NewECRecord(diffEpochIndex)
		ecRecord.SetECR(epoch.MerkleRoot{})
		ecRecord.SetPrevEC(epoch.MerkleRoot{})

		header = &ledger.SnapshotHeader{
			FullEpochIndex: fullEpochIndex,
			DiffEpochIndex: diffEpochIndex,
			LatestECRecord: ecRecord,
		}

		return
	}

	// prepare outputsWithMetadata
	createsOutputsWithMetadatas(t, 110)
	i := 0
	utxoStatesProd := func() *ledger.OutputWithMetadata {
		if i == len(outputsWithMetadata) {
			return nil
		}

		o := outputsWithMetadata[i]
		i++
		return o
	}

	// prepare epoch diffs
	createsEpochDiffs(fullEpochIndex, diffEpochIndex)
	k := 0
	epochDiffsProd := func() (diffs *ledger.EpochDiff) {
		if i == len(epochDiffs) {
			return nil
		}

		d := epochDiffs[k]
		k++
		return d
	}

	solidEntryPoints = createSolidEntryPoints(t, fullEpochIndex, diffEpochIndex)
	j := 0
	sepsProd := func() (s *SolidEntryPoints) {
		if j == len(solidEntryPoints) {
			return nil
		}
		s = solidEntryPoints[j]
		j++
		return s
	}

	header, err := CreateSnapshot(snapshotFileName, headerProd, sepsProd, utxoStatesProd, epochDiffsProd)
	require.NoError(t, err)

	return header
}

func readSnapshot(t *testing.T) (header *ledger.SnapshotHeader, seps []*SolidEntryPoints, states []*ledger.OutputWithMetadata, epochDiffs []*ledger.EpochDiff) {
	outputWithMetadataConsumer := func(outputWithMetadatas []*ledger.OutputWithMetadata) {
		states = append(states, outputWithMetadatas...)
	}
	epochDiffsConsumer := func(_ epoch.Index, diffs *ledger.EpochDiff) {
		epochDiffs = append(epochDiffs, diffs)
	}
	headerConsumer := func(h *ledger.SnapshotHeader) {
		header = h
	}
	sepsConsumer := func(s *SolidEntryPoints) {
		seps = append(seps, s)
	}

	err := LoadSnapshot(snapshotFileName, headerConsumer, sepsConsumer, outputWithMetadataConsumer, epochDiffsConsumer)
	require.NoError(t, err)
	return
}

func createsEpochDiffs(fullEpochIndex, diffEpochIndex epoch.Index) {
	l, size := 0, 10
	for i := fullEpochIndex + 1; i <= diffEpochIndex; i++ {
		spent, created := make([]*ledger.OutputWithMetadata, 0), make([]*ledger.OutputWithMetadata, 0)
		spent = append(spent, outputsWithMetadata[l*size:(l+1)*size]...)
		created = append(created, outputsWithMetadata[(l+1)*size:(l+2)*size]...)

		epochDiffs = append(epochDiffs, ledger.NewEpochDiff(spent, created))
		l += 2
	}
}

func createSolidEntryPoints(t *testing.T, fullEpochIndex, diffEpochIndex epoch.Index) (seps []*SolidEntryPoints) {
	for i := fullEpochIndex; i <= diffEpochIndex; i++ {
		sep := &SolidEntryPoints{EI: i, Seps: make([]tangleold.BlockID, 0)}
		for j := 0; j < 101; j++ {
			var b tangleold.BlockID
			require.NoError(t, b.FromRandomness(i))
			sep.Seps = append(sep.Seps, b)
		}
		seps = append(seps, sep)
	}
	return
}

func createManaDistribution(totalTokensToPledge uint64) (manaDistribution map[identity.ID]uint64) {
	manaDistribution = make(map[identity.ID]uint64)
	for _, node := range nodesToPledge {
		nodeID, err := identity.DecodeIDBase58(node)
		if err != nil {
			panic("failed to decode node id: " + err.Error())
		}

		manaDistribution[nodeID] = totalTokensToPledge / uint64(len(nodesToPledge))
	}

	return manaDistribution
}

var outputCounter uint16 = 1

func createsOutputsWithMetadatas(t *testing.T, total int) {
	now := time.Now()
	for i := 0; i < total; {
		for nodeID, value := range manaDistribution {
			if i >= total {
				break
			}
			// pledge to ID but send funds to random address
			output, outputMetadata := createOutput(devnetvm.NewED25519Address(ed25519.GenerateKeyPair().PublicKey), value, nodeID, now)
			outputsWithMetadata = append(outputsWithMetadata, ledger.NewOutputWithMetadata(output.ID(), output, outputMetadata.CreationTime(), outputMetadata.ConsensusManaPledgeID(), outputMetadata.AccessManaPledgeID()))
			i++
		}
	}

}

func createOutput(address devnetvm.Address, tokenAmount uint64, pledgeID identity.ID, creationTime time.Time) (output devnetvm.Output, outputMetadata *ledger.OutputMetadata) {
	output = devnetvm.NewSigLockedColoredOutput(devnetvm.NewColoredBalances(map[devnetvm.Color]uint64{
		devnetvm.ColorIOTA: tokenAmount,
	}), address)
	output.SetID(utxo.NewOutputID(utxo.EmptyTransactionID, outputCounter))
	outputCounter++

	outputMetadata = ledger.NewOutputMetadata(output.ID())
	outputMetadata.SetConfirmationState(confirmation.Confirmed)
	outputMetadata.SetAccessManaPledgeID(pledgeID)
	outputMetadata.SetConsensusManaPledgeID(pledgeID)
	outputMetadata.SetCreationTime(creationTime)

	return output, outputMetadata
}

func compareSnapshotHeader(t *testing.T, created, unmarshal *ledger.SnapshotHeader) {
	assert.Equal(t, created.FullEpochIndex, unmarshal.FullEpochIndex)
	assert.Equal(t, created.DiffEpochIndex, unmarshal.DiffEpochIndex)
	assert.Equal(t, created.OutputWithMetadataCount, unmarshal.OutputWithMetadataCount)
	oLatestECRecordBytes, err := created.LatestECRecord.Bytes()
	require.NoError(t, err)
	nLatestECRecordBytes, err := unmarshal.LatestECRecord.Bytes()
	require.NoError(t, err)
	assert.ElementsMatch(t, oLatestECRecordBytes, nLatestECRecordBytes)
}

func compareSolidEntryPoints(t *testing.T, created, unmarshal []*SolidEntryPoints) {
	assert.Equal(t, len(created), len(unmarshal))
	for i := 0; i < len(created); i++ {
		assert.Equal(t, created[i].EI, unmarshal[i].EI)
		for j := 0; j < len(created[i].Seps); j++ {
			ob := created[i].Seps[j].Bytes()
			rb := unmarshal[i].Seps[j].Bytes()
			assert.ElementsMatch(t, ob, rb)
		}
	}
}

func compareOutputWithMetadataSlice(t *testing.T, created, unmarshal []*ledger.OutputWithMetadata) {
	assert.Equal(t, len(created), len(unmarshal))
	for i := 0; i < len(created); i++ {
		ob, err := created[i].Bytes()
		require.NoError(t, err)
		rb, err := unmarshal[i].Bytes()
		require.NoError(t, err)
		assert.ElementsMatch(t, ob, rb)
	}
}

func compareEpochDiffs(t *testing.T, created, unmarshal []*ledger.EpochDiff) {
	assert.Equal(t, len(created), len(unmarshal))
	for i, diffs := range created {
		uDiffs := unmarshal[i]

		compareOutputWithMetadataSlice(t, diffs.Spent(), uDiffs.Spent())
		compareOutputWithMetadataSlice(t, diffs.Created(), uDiffs.Created())
	}
}
