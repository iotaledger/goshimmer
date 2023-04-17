package chainmanager

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/hive.go/ds/types"
	"github.com/iotaledger/hive.go/lo"
)

func TestManager(t *testing.T) {
	tf := NewTestFramework(t)
	tf.CreateCommitment("1", "Genesis")
	tf.CreateCommitment("2", "1")
	tf.CreateCommitment("3", "2")
	tf.CreateCommitment("4", "3")
	tf.CreateCommitment("4*", "3")
	tf.CreateCommitment("1*", "Genesis")
	tf.CreateCommitment("2*", "1*")
	tf.CreateCommitment("5*", "4*")
	tf.CreateCommitment("6*", "5*")
	tf.CreateCommitment("7*", "6*")
	tf.CreateCommitment("8*", "7*")

	forkDetected := make(chan struct{}, 1)
	tf.Instance.Events.ForkDetected.Hook(func(fork *Fork) {
		// The ForkDetected event should only be triggered once and only if the fork is deep enough
		require.Equal(t, fork.Commitment.ID(), tf.SlotCommitment("7*"))
		require.Equal(t, fork.ForkingPoint.ID(), tf.SlotCommitment("4*"))
		forkDetected <- struct{}{}
		close(forkDetected) // closing channel here so that we are sure no second event with the same data is triggered
	})

	expectedChainMappings := map[string]string{
		"Genesis": "Genesis",
	}

	{
		isSolid, chain := tf.ProcessCommitment("1")
		require.True(t, isSolid)
		tf.AssertChainIsAlias(chain, "Genesis")
		tf.AssertChainState(lo.MergeMaps(expectedChainMappings, map[string]string{
			"1": "Genesis",
		}))
	}

	{
		isSolid, chain := tf.ProcessCommitment("1")
		require.True(t, isSolid)
		tf.AssertChainIsAlias(chain, "Genesis")
		tf.AssertChainState(lo.MergeMaps(expectedChainMappings, map[string]string{}))
	}

	{
		isSolid, chain := tf.ProcessCommitmentFromOtherSource("1*")
		require.True(t, isSolid)
		tf.AssertChainIsAlias(chain, "1*")
		tf.AssertChainState(lo.MergeMaps(expectedChainMappings, map[string]string{
			"1*": "1*",
		}))
	}

	{
		isSolid, chain := tf.ProcessCommitmentFromOtherSource("4")
		require.False(t, isSolid)
		tf.AssertChainIsAlias(chain, "")
		tf.AssertChainState(lo.MergeMaps(expectedChainMappings, map[string]string{
			"4": "",
		}))
	}

	{
		isSolid, chain := tf.ProcessCommitmentFromOtherSource("4*")
		require.False(t, isSolid)
		tf.AssertChainIsAlias(chain, "4*")
		tf.AssertChainState(lo.MergeMaps(expectedChainMappings, map[string]string{
			"4*": "4*",
		}))
	}

	{
		isSolid, chain := tf.ProcessCommitmentFromOtherSource("3")
		require.False(t, isSolid)
		tf.AssertChainIsAlias(chain, "")
		tf.AssertChainState(lo.MergeMaps(expectedChainMappings, map[string]string{
			"3": "",
		}))
	}

	{
		isSolid, chain := tf.ProcessCommitment("2")
		require.True(t, isSolid)
		tf.AssertChainIsAlias(chain, "Genesis")
		tf.AssertChainState(lo.MergeMaps(expectedChainMappings, map[string]string{
			"2": "Genesis",
			"3": "Genesis",
			"4": "Genesis",
		}))
	}

	{
		isSolid, chain := tf.ProcessCommitmentFromOtherSource("5*")
		require.True(t, isSolid)
		tf.AssertChainIsAlias(chain, "4*")
		tf.AssertChainState(lo.MergeMaps(expectedChainMappings, map[string]string{
			"5*": "4*",
		}))
	}

	{
		isSolid, chain := tf.ProcessCommitmentFromOtherSource("6*")
		require.True(t, isSolid)
		tf.AssertChainIsAlias(chain, "4*")
		tf.AssertChainState(lo.MergeMaps(expectedChainMappings, map[string]string{
			"6*": "4*",
		}))
	}

	{
		isSolid, chain := tf.ProcessCommitmentFromOtherSource("7*")
		require.True(t, isSolid)
		tf.AssertChainIsAlias(chain, "4*")
		tf.AssertChainState(lo.MergeMaps(expectedChainMappings, map[string]string{
			"7*": "4*",
		}))
	}

	{
		isSolid, chain := tf.ProcessCommitmentFromOtherSource("8*")
		require.True(t, isSolid)
		tf.AssertChainIsAlias(chain, "4*")
		tf.AssertChainState(lo.MergeMaps(expectedChainMappings, map[string]string{
			"8*": "4*",
		}))
	}

	{
		commitments, err := tf.Instance.Commitments(tf.SlotCommitment("8*"), 9)
		require.NoError(t, err)
		tf.AssertEqualChainCommitments(commitments,
			"8*",
			"7*",
			"6*",
			"5*",
			"4*",
			"3",
			"2",
			"1",
			"Genesis",
		)
	}

	{
		commitments, err := tf.Instance.Commitments(tf.SlotCommitment("8*"), 10)
		require.Error(t, err)
		require.EqualValues(t, []*Commitment(nil), commitments)
	}

	{
		require.Nil(t, tf.Instance.Chain(commitment.NewID(1, []byte{255, 255})))
	}

	require.Eventually(t, func() bool {
		select {
		case <-forkDetected:
			return true
		default:
			return false
		}
	}, 1*time.Second, 10*time.Millisecond)
}

func TestManagerForkDetectedAgain(t *testing.T) {
	tf := NewTestFramework(t)
	tf.CreateCommitment("1", "Genesis")
	tf.CreateCommitment("2", "1")
	tf.CreateCommitment("3", "2")
	tf.CreateCommitment("4", "3")
	tf.CreateCommitment("4*", "3")
	tf.CreateCommitment("1*", "Genesis")
	tf.CreateCommitment("2*", "1*")
	tf.CreateCommitment("5*", "4*")
	tf.CreateCommitment("6*", "5*")
	tf.CreateCommitment("7*", "6*")
	tf.CreateCommitment("8*", "7*")
	tf.CreateCommitment("9*", "8*")

	forkRedetected := make(chan struct{}, 1)
	expectedForks := map[commitment.ID]types.Empty{
		tf.SlotCommitment("7*"): types.Void,
		tf.SlotCommitment("9*"): types.Void,
	}
	tf.Instance.Events.ForkDetected.Hook(func(fork *Fork) {
		if _, has := expectedForks[fork.Commitment.ID()]; !has {
			t.Fatalf("unexpected fork at: %s", fork.Commitment.ID())
		}
		t.Logf("fork detected at %s", fork.Commitment.ID())
		delete(expectedForks, fork.Commitment.ID())

		require.Equal(t, fork.ForkingPoint.ID(), tf.SlotCommitment("4*"))
		if len(expectedForks) == 0 {
			forkRedetected <- struct{}{}
		}
	})

	{
		tf.ProcessCommitment("1")
		tf.ProcessCommitmentFromOtherSource("1*")
		tf.ProcessCommitmentFromOtherSource("4")
		tf.ProcessCommitmentFromOtherSource("4*")
		tf.ProcessCommitmentFromOtherSource("3")
		tf.ProcessCommitment("2")
		tf.ProcessCommitmentFromOtherSource("5*")
		tf.ProcessCommitmentFromOtherSource("6*")
		tf.ProcessCommitmentFromOtherSource("7*")
		tf.ProcessCommitmentFromOtherSource("8*")
	}

	{
		commitments, err := tf.Instance.Commitments(tf.SlotCommitment("8*"), 9)
		require.NoError(t, err)
		tf.AssertEqualChainCommitments(commitments,
			"8*",
			"7*",
			"6*",
			"5*",
			"4*",
			"3",
			"2",
			"1",
			"Genesis",
		)
	}

	{
		require.Nil(t, tf.Instance.Chain(commitment.NewID(1, []byte{255, 255})))
	}

	// We now evict at 7 so that we forget about the fork we had before
	{
		tf.Instance.EvictUntil(8)
	}

	// Processing the next commitment should trigger the event again
	{
		isSolid, chain := tf.ProcessCommitmentFromOtherSource("9*")
		require.False(t, isSolid, "commitment should not be solid, as we evicted until epoch 7")
		require.Nil(t, chain, "commitment chain should be nil, as we evicted until epoch 7")
	}
}

func TestEvaluateAgainstRootCommitment(t *testing.T) {
	rootCommitment := commitment.New(1, commitment.NewID(1, []byte{9}), types.Identifier{}, 0)
	m := &Manager{
		rootCommitment: NewCommitment(rootCommitment.ID()),
	}

	m.rootCommitment.PublishCommitment(rootCommitment)

	isBelow, isRootCommitment := m.evaluateAgainstRootCommitment(commitment.New(0, commitment.NewID(0, []byte{}), types.Identifier{}, 0))
	require.True(t, isBelow, "commitment with index 0 should be below root commitment")
	require.False(t, isRootCommitment, "commitment with index 0 should not be the root commitment")

	isBelow, isRootCommitment = m.evaluateAgainstRootCommitment(rootCommitment)
	require.True(t, isBelow, "commitment with index 1 should be below root commitment")
	require.True(t, isRootCommitment, "commitment with index 1 should be the root commitment")

	isBelow, isRootCommitment = m.evaluateAgainstRootCommitment(commitment.New(1, commitment.NewID(1, []byte{1}), types.Identifier{}, 0))
	require.True(t, isBelow, "commitment with index 1 should be below root commitment")
	require.False(t, isRootCommitment, "commitment with index 1 should be the root commitment")

	isBelow, isRootCommitment = m.evaluateAgainstRootCommitment(commitment.New(1, commitment.NewID(1, []byte{9}), types.Identifier{}, 0))
	require.True(t, isBelow, "commitment with index 1 should be below root commitment")
	require.True(t, isRootCommitment, "commitment with index 1 should be the root commitment")

	isBelow, isRootCommitment = m.evaluateAgainstRootCommitment(commitment.New(2, commitment.NewID(2, []byte{}), types.Identifier{}, 0))
	require.False(t, isBelow, "commitment with index 2 should not be below root commitment")
	require.False(t, isRootCommitment, "commitment with index 2 should not be the root commitment")
}

func TestProcessCommitment(t *testing.T) {
	tf := NewTestFramework(t)
	tf.CreateCommitment("1", "Genesis")
	tf.CreateCommitment("2", "1")

	expectedChainMappings := map[string]string{
		"Genesis": "Genesis",
	}

	{
		isSolid, chain := tf.ProcessCommitment("1")
		require.True(t, isSolid)
		tf.AssertChainIsAlias(chain, "Genesis")
		tf.AssertChainState(lo.MergeMaps(expectedChainMappings, map[string]string{
			"1": "Genesis",
		}))
	}
	{
		isSolid, chain := tf.ProcessCommitment("2")
		require.True(t, isSolid)
		tf.AssertChainIsAlias(chain, "Genesis")
		tf.AssertChainState(lo.MergeMaps(expectedChainMappings, map[string]string{
			"2": "Genesis",
		}))
	}

	fmt.Println("------- root commitment is now 2 -------")
	tf.Instance.SetRootCommitment(tf.commitment("2"))
	tf.Instance.EvictUntil(2 - 1)

	{
		require.Equal(t, tf.commitment("2").ID(), tf.Instance.rootCommitment.ID())
	}

	// Should not be processed after 2 becomes rootCommitment
	tf.CreateCommitment("1*", "Genesis")
	tf.CreateCommitment("2*", "1*")
	tf.CreateCommitment("3*", "2*")
	tf.CreateCommitment("4*", "3*")
	tf.CreateCommitment("2+", "1")
	{
		{
			isSolid, chain := tf.ProcessCommitment("1*")
			require.False(t, isSolid)
			require.Nil(t, chain)
			tf.AssertForkDetectedCount(0)
			tf.AssertCommitmentMissingCount(0)
			tf.AssertMissingCommitmentReceivedCount(0)
			tf.AssertCommitmentBelowRootCount(1)
		}
		{
			isSolid, chain := tf.ProcessCommitment("2*")
			require.False(t, isSolid)
			require.Nil(t, chain)
			tf.AssertForkDetectedCount(0)
			tf.AssertCommitmentMissingCount(0)
			tf.AssertMissingCommitmentReceivedCount(0)
			tf.AssertCommitmentBelowRootCount(2)
		}
		{
			isSolid, chain := tf.ProcessCommitment("3*")
			require.False(t, isSolid)
			require.Nil(t, chain)
			tf.AssertForkDetectedCount(0)
			tf.AssertCommitmentMissingCount(1)
			tf.AssertMissingCommitmentReceivedCount(0)
			tf.AssertCommitmentBelowRootCount(2)
		}
		{
			isSolid, chain := tf.ProcessCommitment("2+")
			require.False(t, isSolid)
			require.Nil(t, chain)
			tf.AssertForkDetectedCount(0)
			tf.AssertCommitmentMissingCount(1)
			tf.AssertMissingCommitmentReceivedCount(0)
			tf.AssertCommitmentBelowRootCount(3)
		}
	}

	// Should be processed after 2 becomes rootCommitment
	tf.CreateCommitment("3", "2")
	tf.CreateCommitment("4", "3")
	{
		{
			isSolid, chain := tf.ProcessCommitment("2")
			require.True(t, isSolid)
			tf.AssertChainIsAlias(chain, "Genesis")
			tf.AssertChainState(lo.MergeMaps(expectedChainMappings, map[string]string{
				"Genesis": "evicted",
				"1":       "evicted",
				"2":       "Genesis",
			}))
			tf.AssertForkDetectedCount(0)
			tf.AssertCommitmentMissingCount(1)
			tf.AssertMissingCommitmentReceivedCount(0)
			tf.AssertCommitmentBelowRootCount(3)
		}
		{
			isSolid, chain := tf.ProcessCommitment("4")
			require.False(t, isSolid)
			require.Nil(t, chain, "Genesis")
			tf.AssertChainState(lo.MergeMaps(expectedChainMappings, map[string]string{
				"4": "",
			}))
			tf.AssertForkDetectedCount(0)
			tf.AssertCommitmentMissingCount(2)
			tf.AssertMissingCommitmentReceivedCount(0)
			tf.AssertCommitmentBelowRootCount(3)
		}
		{
			isSolid, chain := tf.ProcessCommitment("3")
			require.True(t, isSolid)
			tf.AssertChainIsAlias(chain, "Genesis")
			tf.AssertChainState(lo.MergeMaps(expectedChainMappings, map[string]string{
				"3": "Genesis",
				"4": "Genesis",
			}))
			tf.AssertForkDetectedCount(0)
			tf.AssertCommitmentMissingCount(2)
			tf.AssertMissingCommitmentReceivedCount(1)
			tf.AssertCommitmentBelowRootCount(3)
		}
		{
			isSolid, chain := tf.ProcessCommitment("4")
			require.True(t, isSolid)
			tf.AssertChainIsAlias(chain, "Genesis")
			tf.AssertChainState(lo.MergeMaps(expectedChainMappings, map[string]string{}))
			tf.AssertForkDetectedCount(0)
			tf.AssertCommitmentMissingCount(2)
			tf.AssertMissingCommitmentReceivedCount(1)
			tf.AssertCommitmentBelowRootCount(3)
		}
	}
}
