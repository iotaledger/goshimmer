package epoch

import (
	"testing"

	"github.com/iotaledger/hive.go/core/generics/lo"
)

func TestCommitmentChainManager_ChainFromBlock(t *testing.T) {
	tf := NewTestFramework(t)
	tf.AddCommitment("1", "genesis")
	tf.AddCommitment("2", "1")
	tf.AddCommitment("3", "2")
	tf.AddCommitment("4", "3")
	tf.AddCommitment("1*", "genesis")
	tf.AddCommitment("2*", "1*")

	expectedChainMappings := make(map[string]string)

	{
		tf.AssertChain(tf.Chain("1"), "genesis")
		tf.AssertChains(lo.MergeMaps(expectedChainMappings, map[string]string{
			"1": "genesis",
		}))
	}

	{
		tf.AssertChain(tf.Chain("1"), "genesis")
		tf.AssertChains(lo.MergeMaps(expectedChainMappings, map[string]string{
			"1": "genesis",
		}))
	}

	{
		tf.AssertChain(tf.Chain("1*"), "1*")
		tf.AssertChains(lo.MergeMaps(expectedChainMappings, map[string]string{
			"1*": "1*",
		}))
	}

	{
		tf.AssertChain(tf.Chain("4"), "")
		tf.AssertChains(lo.MergeMaps(expectedChainMappings, map[string]string{
			"4": "",
		}))
	}

	{
		tf.AssertChain(tf.Chain("3"), "")
		tf.AssertChains(lo.MergeMaps(expectedChainMappings, map[string]string{
			"3": "",
		}))
	}

	{
		tf.AssertChain(tf.Chain("2"), "genesis")
		tf.AssertChains(lo.MergeMaps(expectedChainMappings, map[string]string{
			"2": "genesis",
			"3": "genesis",
			"4": "genesis",
		}))
	}
}
