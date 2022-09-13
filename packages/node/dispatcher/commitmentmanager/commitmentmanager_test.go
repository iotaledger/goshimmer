package commitmentmanager

import (
	"testing"

	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

func TestCommitmentManager(t *testing.T) {
	tf := NewTestFramework(t)
	tf.CreateCommitment("1", "genesis")
	tf.CreateCommitment("2", "1")
	tf.CreateCommitment("3", "2")
	tf.CreateCommitment("4", "3")
	tf.CreateCommitment("4*", "3")
	tf.CreateCommitment("1*", "genesis")
	tf.CreateCommitment("2*", "1*")

	expectedChainMappings := make(map[string]string)

	{
		chain, wasForked := tf.ProcessCommitment("1")
		require.False(t, wasForked)
		tf.AssertChainIsAlias(chain, "genesis")
		tf.AssertChainState(lo.MergeMaps(expectedChainMappings, map[string]string{
			"genesis": "genesis",
			"1":       "genesis",
		}))
	}

	{
		chain, wasForked := tf.ProcessCommitment("1")
		require.False(t, wasForked)
		tf.AssertChainIsAlias(chain, "genesis")
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
		tf.AssertChainIsAlias(chain, "genesis")
		tf.AssertChainState(lo.MergeMaps(expectedChainMappings, map[string]string{
			"2": "genesis",
			"3": "genesis",
			"4": "genesis",
		}))
	}

	{
		commitments, err := tf.ChainManager.Commitments(tf.EC("4*"), 5)
		require.NoError(t, err)
		require.EqualValues(t, []*Commitment{
			tf.ChainManager.Commitment(tf.EC("4*")),
			tf.ChainManager.Commitment(tf.EC("3")),
			tf.ChainManager.Commitment(tf.EC("2")),
			tf.ChainManager.Commitment(tf.EC("1")),
			tf.ChainManager.Commitment(tf.EC("genesis")),
		}, commitments)
	}

	{
		commitments, err := tf.ChainManager.Commitments(tf.EC("4*"), 6)
		require.Error(t, err)
		require.EqualValues(t, ([]*Commitment)(nil), commitments)
	}

	{
		require.Nil(t, tf.ChainManager.Chain(epoch.EC{255, 255}))
	}
}
