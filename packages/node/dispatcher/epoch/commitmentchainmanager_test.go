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
	tf.AddCommitment("1*", "genesis")
	tf.AddCommitment("2*", "1*")

	expectedChainMappings := make(map[string]string)

	{
		chain := tf.ProcessCommitment("1")
		tf.AssertChain(chain, "genesis")
		tf.AssertChains(lo.MergeMaps(expectedChainMappings, map[string]string{
			"1": "genesis",
		}))
	}

	{
		chain := tf.ProcessCommitment("1*")
		tf.AssertChain(chain, "1*")
		tf.AssertChains(lo.MergeMaps(expectedChainMappings, map[string]string{
			"1*": "1*",
		}))
	}

	{
		chain := tf.ProcessCommitment("3")
		tf.AssertChain(chain, "")
		tf.AssertChains(lo.MergeMaps(expectedChainMappings, map[string]string{
			"3": "",
		}))
	}

	{
		chain := tf.ProcessCommitment("2")
		tf.AssertChain(chain, "genesis")
		tf.AssertChains(lo.MergeMaps(expectedChainMappings, map[string]string{
			"2": "genesis",
			"3": "genesis",
		}))
	}
}
