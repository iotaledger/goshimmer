package chain

import (
	"testing"

	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
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

	expectedChainMappings := map[string]string{
		"Genesis": "Genesis",
	}

	{
		chain, wasForked := tf.ProcessCommitment("1")
		require.False(t, wasForked)
		tf.AssertChainIsAlias(chain, "Genesis")
		tf.AssertChainState(lo.MergeMaps(expectedChainMappings, map[string]string{
			"1": "Genesis",
		}))
	}

	{
		chain, wasForked := tf.ProcessCommitment("1")
		require.False(t, wasForked)
		tf.AssertChainIsAlias(chain, "Genesis")
		tf.AssertChainState(lo.MergeMaps(expectedChainMappings, map[string]string{}))
	}

	{
		chain, wasForked := tf.ProcessCommitment("1*")
		require.True(t, wasForked)
		tf.AssertChainIsAlias(chain, "1*")
		tf.AssertChainState(lo.MergeMaps(expectedChainMappings, map[string]string{
			"1*": "1*",
		}))
	}

	{
		chain, wasForked := tf.ProcessCommitment("4")
		require.False(t, wasForked)
		tf.AssertChainIsAlias(chain, "")
		tf.AssertChainState(lo.MergeMaps(expectedChainMappings, map[string]string{
			"4": "",
		}))
	}

	{
		chain, wasForked := tf.ProcessCommitment("4*")
		require.True(t, wasForked)
		tf.AssertChainIsAlias(chain, "4*")
		tf.AssertChainState(lo.MergeMaps(expectedChainMappings, map[string]string{
			"4*": "4*",
		}))
	}

	{
		chain, wasForked := tf.ProcessCommitment("3")
		require.False(t, wasForked)
		tf.AssertChainIsAlias(chain, "")
		tf.AssertChainState(lo.MergeMaps(expectedChainMappings, map[string]string{
			"3": "",
		}))
	}

	{
		chain, wasForked := tf.ProcessCommitment("2")
		require.False(t, wasForked)
		tf.AssertChainIsAlias(chain, "Genesis")
		tf.AssertChainState(lo.MergeMaps(expectedChainMappings, map[string]string{
			"2": "Genesis",
			"3": "Genesis",
			"4": "Genesis",
		}))
	}

	{
		commitments, err := tf.Manager.Commitments(tf.EC("4*"), 5)
		require.NoError(t, err)
		require.EqualValues(t, []*Commitment{
			tf.Manager.Commitment(tf.EC("4*")),
			tf.Manager.Commitment(tf.EC("3")),
			tf.Manager.Commitment(tf.EC("2")),
			tf.Manager.Commitment(tf.EC("1")),
			tf.Manager.Commitment(tf.EC("Genesis")),
		}, commitments)
	}

	{
		commitments, err := tf.Manager.Commitments(tf.EC("4*"), 6)
		require.Error(t, err)
		require.EqualValues(t, ([]*Commitment)(nil), commitments)
	}

	{
		require.Nil(t, tf.Manager.Chain(commitment.ID{255, 255}))
	}
}
