package chainmanager

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/types"
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
	event.Hook(tf.Instance.Events.ForkDetected, func(fork *Fork) {
		// The ForkDetected event should only be triggered once and only if the fork is deep enough
		require.Equal(t, fork.Commitment.ID(), tf.EC("7*"))
		require.Equal(t, fork.ForkingPoint.ID(), tf.EC("4*"))
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
		commitments, err := tf.Instance.Commitments(tf.EC("8*"), 9)
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
		commitments, err := tf.Instance.Commitments(tf.EC("8*"), 10)
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
		tf.EC("7*"): types.Void,
		tf.EC("9*"): types.Void,
	}
	event.Hook(tf.Instance.Events.ForkDetected, func(fork *Fork) {
		if _, has := expectedForks[fork.Commitment.ID()]; !has {
			t.Fatalf("unexpected fork at: %s", fork.Commitment.ID())
		}
		t.Logf("fork detected at %s", fork.Commitment.ID())
		delete(expectedForks, fork.Commitment.ID())

		require.Equal(t, fork.ForkingPoint.ID(), tf.EC("4*"))
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
		commitments, err := tf.Instance.Commitments(tf.EC("8*"), 9)
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
		tf.Instance.Evict(7)
	}

	// Processing the next commitment should trigger the event again
	{
		tf.ProcessCommitmentFromOtherSource("9*")
	}

	require.Eventually(t, func() bool {
		select {
		case <-forkRedetected:
			return true
		default:
			return false
		}
	}, 1*time.Second, 10*time.Millisecond)
}
