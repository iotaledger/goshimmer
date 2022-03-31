package ledger

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLedger(t *testing.T) {
	testFramework := NewTestFramework()
	testFramework.CreateTransaction("G", 3, "Genesis")
	testFramework.CreateTransaction("TX1", 1, "G.0")
	testFramework.CreateTransaction("TX1*", 1, "G.0")
	testFramework.CreateTransaction("TX2", 1, "G.1")
	testFramework.CreateTransaction("TX2*", 1, "G.1")
	testFramework.CreateTransaction("TX3", 1, "G.2")
	testFramework.CreateTransaction("TX3*", 1, "G.2")
	testFramework.CreateTransaction("TX4", 1, "TX1.0", "TX2.0")
	testFramework.CreateTransaction("TX5", 1, "TX4.0")
	testFramework.CreateTransaction("TX5*", 1, "TX4.0")
	testFramework.CreateTransaction("TX6", 1, "TX5.0", "TX3.0")
	testFramework.CreateTransaction("TX7", 1, "TX6.0")
	testFramework.CreateTransaction("TX7*", 1, "TX6.0")
	testFramework.CreateTransaction("TX8", 1, "TX7.0")

	{
		for _, txAlias := range []string{"G", "TX1", "TX1*", "TX2", "TX2*", "TX3", "TX3*", "TX4", "TX5", "TX6", "TX7", "TX8"} {
			assert.NoError(t, testFramework.IssueTransaction(txAlias))
		}

		testFramework.AssertBranchIDs(t, map[string][]string{
			"G":    {"MasterBranch"},
			"TX1":  {"TX1"},
			"TX1*": {"TX1*"},
			"TX2":  {"TX2"},
			"TX2*": {"TX2*"},
			"TX3":  {"TX3"},
			"TX3*": {"TX3*"},
			"TX4":  {"TX1", "TX2"},
			"TX5":  {"TX1", "TX2"},
			"TX6":  {"TX1", "TX2", "TX3"},
			"TX7":  {"TX1", "TX2", "TX3"},
			"TX8":  {"TX1", "TX2", "TX3"},
		})

		testFramework.AssertBranchDAG(t, map[string][]string{
			"TX1":  {"MasterBranch"},
			"TX1*": {"MasterBranch"},
			"TX2":  {"MasterBranch"},
			"TX2*": {"MasterBranch"},
			"TX3":  {"MasterBranch"},
			"TX3*": {"MasterBranch"},
		})
	}

	{
		assert.NoError(t, testFramework.IssueTransaction("TX7*"))

		testFramework.AssertBranchIDs(t, map[string][]string{
			"G":    {"MasterBranch"},
			"TX1":  {"TX1"},
			"TX1*": {"TX1*"},
			"TX2":  {"TX2"},
			"TX2*": {"TX2*"},
			"TX3":  {"TX3"},
			"TX3*": {"TX3*"},
			"TX4":  {"TX1", "TX2"},
			"TX5":  {"TX1", "TX2"},
			"TX6":  {"TX1", "TX2", "TX3"},
			"TX7":  {"TX7"},
			"TX7*": {"TX7*"},
			"TX8":  {"TX7"},
		})

		testFramework.AssertBranchDAG(t, map[string][]string{
			"TX1":  {"MasterBranch"},
			"TX1*": {"MasterBranch"},
			"TX2":  {"MasterBranch"},
			"TX2*": {"MasterBranch"},
			"TX3":  {"MasterBranch"},
			"TX3*": {"MasterBranch"},
			"TX7":  {"TX1", "TX2", "TX3"},
			"TX7*": {"TX1", "TX2", "TX3"},
		})
	}

	{
		assert.NoError(t, testFramework.IssueTransaction("TX5*"))

		testFramework.AssertBranchIDs(t, map[string][]string{
			"G":    {"MasterBranch"},
			"TX1":  {"TX1"},
			"TX1*": {"TX1*"},
			"TX2":  {"TX2"},
			"TX2*": {"TX2*"},
			"TX3":  {"TX3"},
			"TX3*": {"TX3*"},
			"TX4":  {"TX1", "TX2"},
			"TX5":  {"TX5"},
			"TX5*": {"TX5*"},
			"TX6":  {"TX5", "TX3"},
			"TX7":  {"TX7"},
			"TX7*": {"TX7*"},
			"TX8":  {"TX7"},
		})

		testFramework.AssertBranchDAG(t, map[string][]string{
			"TX1":  {"MasterBranch"},
			"TX1*": {"MasterBranch"},
			"TX2":  {"MasterBranch"},
			"TX2*": {"MasterBranch"},
			"TX3":  {"MasterBranch"},
			"TX3*": {"MasterBranch"},
			"TX5":  {"TX1", "TX2"},
			"TX5*": {"TX1", "TX2"},
			"TX7":  {"TX5", "TX3"},
			"TX7*": {"TX5", "TX3"},
		})
	}
}
