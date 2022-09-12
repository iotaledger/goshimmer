package epoch

import (
	"testing"
)

func TestCommitmentChainManager_ChainFromBlock(t *testing.T) {
	tf := NewTestFramework(t)
	tf.AddCommitment("1", "genesis")
	tf.AddCommitment("2", "1")
	tf.AddCommitment("3", "2")
	tf.AddCommitment("1*", "genesis")
	tf.AddCommitment("2*", "1*")

	{
		chain := tf.ProcessCommitment("1")
		tf.AssertChain(chain, "genesis")
	}

	{
		chain := tf.ProcessCommitment("1*")
		tf.AssertChain(chain, "1*")
	}

	{
		chain := tf.ProcessCommitment("3")
		tf.AssertChain(chain, "")
	}

	{
		chain := tf.ProcessCommitment("2")
		tf.AssertChain(chain, "genesis")
	}
}
