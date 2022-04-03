package ledger

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/ledger/branchdag"
)

func TestLedger_BookInOrder(t *testing.T) {
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

// See scenario at img/ledger_test_SetBranchConfirmed.png
func TestLedger_SetBranchConfirmed(t *testing.T) {
	testFramework := NewTestFramework()

	// Step 1: Bottom Layer
	testFramework.CreateTransaction("G", 3, "Genesis")
	testFramework.CreateTransaction("TXA", 1, "G.0")
	testFramework.CreateTransaction("TXB", 1, "G.0")
	testFramework.CreateTransaction("TXC", 1, "G.1")
	testFramework.CreateTransaction("TXD", 1, "G.1")
	testFramework.CreateTransaction("TXH", 1, "G.2")
	testFramework.CreateTransaction("TXI", 1, "G.2")
	// Step 2: Middle Layer
	testFramework.CreateTransaction("TXE", 1, "TXA.0", "TXC.0")
	// Step 3: Top Layer
	testFramework.CreateTransaction("TXF", 1, "TXE.0")
	// Step 4: Top Layer
	testFramework.CreateTransaction("TXG", 1, "TXE.0")
	// Step 5: TopTop Layer
	testFramework.CreateTransaction("TXL", 1, "TXG.0", "TXH.0")
	// Step 6: TopTopTOP Layer
	testFramework.CreateTransaction("TXM", 1, "TXL.0")

	// Mark A as Confirmed
	{
		for _, txAlias := range []string{"G", "TXA", "TXB", "TXC", "TXD", "TXH", "TXI"} {
			assert.NoError(t, testFramework.IssueTransaction(txAlias))
		}
		require.True(t, testFramework.Ledger.BranchDAG.SetBranchConfirmed(branchdag.NewBranchID(testFramework.Transaction("TXA").ID())))

		testFramework.AssertBranchIDs(t, map[string][]string{
			"G":   {"MasterBranch"},
			"TXA": {"TXA"},
			"TXB": {"TXB"},
			"TXC": {"TXC"},
			"TXD": {"TXD"},
			"TXH": {"TXH"},
			"TXI": {"TXI"},
		})

		testFramework.AssertBranchDAG(t, map[string][]string{
			"TXA": {"MasterBranch"},
			"TXB": {"MasterBranch"},
			"TXC": {"MasterBranch"},
			"TXD": {"MasterBranch"},
			"TXH": {"MasterBranch"},
			"TXI": {"MasterBranch"},
		})

		assert.Equal(t, branchdag.Confirmed, testFramework.Ledger.BranchDAG.InclusionState(testFramework.BranchIDs("TXA")))
		assert.Equal(t, branchdag.Rejected, testFramework.Ledger.BranchDAG.InclusionState(testFramework.BranchIDs("TXB")))
		assert.Equal(t, branchdag.Pending, testFramework.Ledger.BranchDAG.InclusionState(testFramework.BranchIDs("TXC")))
		assert.Equal(t, branchdag.Pending, testFramework.Ledger.BranchDAG.InclusionState(testFramework.BranchIDs("TXD")))
		assert.Equal(t, branchdag.Pending, testFramework.Ledger.BranchDAG.InclusionState(testFramework.BranchIDs("TXH")))
		assert.Equal(t, branchdag.Pending, testFramework.Ledger.BranchDAG.InclusionState(testFramework.BranchIDs("TXI")))
	}

	// When creating the middle layer the new transaction E should be booked only under its Pending parent C
	{
		assert.NoError(t, testFramework.IssueTransaction("TXE"))

		testFramework.AssertBranchIDs(t, map[string][]string{
			"G":   {"MasterBranch"},
			"TXA": {"TXA"},
			"TXB": {"TXB"},
			"TXC": {"TXC"},
			"TXD": {"TXD"},
			"TXH": {"TXH"},
			"TXI": {"TXI"},
			"TXE": {"TXC"},
		})

		testFramework.AssertBranchDAG(t, map[string][]string{
			"TXA": {"MasterBranch"},
			"TXB": {"MasterBranch"},
			"TXC": {"MasterBranch"},
			"TXD": {"MasterBranch"},
			"TXH": {"MasterBranch"},
			"TXI": {"MasterBranch"},
		})

		assert.Equal(t, branchdag.Confirmed, testFramework.Ledger.BranchDAG.InclusionState(testFramework.BranchIDs("TXA")))
		assert.Equal(t, branchdag.Rejected, testFramework.Ledger.BranchDAG.InclusionState(testFramework.BranchIDs("TXB")))
		assert.Equal(t, branchdag.Pending, testFramework.Ledger.BranchDAG.InclusionState(testFramework.BranchIDs("TXC")))
		assert.Equal(t, branchdag.Pending, testFramework.Ledger.BranchDAG.InclusionState(testFramework.BranchIDs("TXD")))
		assert.Equal(t, branchdag.Pending, testFramework.Ledger.BranchDAG.InclusionState(testFramework.BranchIDs("TXH")))
		assert.Equal(t, branchdag.Pending, testFramework.Ledger.BranchDAG.InclusionState(testFramework.BranchIDs("TXI")))
	}

	// When creating the first transaction (F) of top layer it should be booked under the Pending parent C
	{
		for _, txAlias := range []string{"TXF"} {
			assert.NoError(t, testFramework.IssueTransaction(txAlias))
		}

		testFramework.AssertBranchIDs(t, map[string][]string{
			"G":   {"MasterBranch"},
			"TXA": {"TXA"},
			"TXB": {"TXB"},
			"TXC": {"TXC"},
			"TXD": {"TXD"},
			"TXH": {"TXH"},
			"TXI": {"TXI"},
			// Branches F & G are spawned by the fork of G
			"TXF": {"TXC"},
		})

		testFramework.AssertBranchDAG(t, map[string][]string{
			"TXA": {"MasterBranch"},
			"TXB": {"MasterBranch"},
			"TXC": {"MasterBranch"},
			"TXD": {"MasterBranch"},
			"TXH": {"MasterBranch"},
			"TXI": {"MasterBranch"},
		})

		assert.Equal(t, branchdag.Confirmed, testFramework.Ledger.BranchDAG.InclusionState(testFramework.BranchIDs("TXA")))
		assert.Equal(t, branchdag.Rejected, testFramework.Ledger.BranchDAG.InclusionState(testFramework.BranchIDs("TXB")))
		assert.Equal(t, branchdag.Pending, testFramework.Ledger.BranchDAG.InclusionState(testFramework.BranchIDs("TXC")))
		assert.Equal(t, branchdag.Pending, testFramework.Ledger.BranchDAG.InclusionState(testFramework.BranchIDs("TXD")))
		assert.Equal(t, branchdag.Pending, testFramework.Ledger.BranchDAG.InclusionState(testFramework.BranchIDs("TXH")))
		assert.Equal(t, branchdag.Pending, testFramework.Ledger.BranchDAG.InclusionState(testFramework.BranchIDs("TXI")))
	}

	// When creating the conflicting TX (G) of the top layer branches F & G are spawned by the fork of G
	{
		for _, txAlias := range []string{"TXG"} {
			assert.NoError(t, testFramework.IssueTransaction(txAlias))
		}

		testFramework.AssertBranchIDs(t, map[string][]string{
			"G":   {"MasterBranch"},
			"TXA": {"TXA"},
			"TXB": {"TXB"},
			"TXC": {"TXC"},
			"TXD": {"TXD"},
			"TXH": {"TXH"},
			"TXI": {"TXI"},
			// Branches F & G are spawned by the fork of G
			"TXF": {"TXF"},
			"TXG": {"TXG"},
		})

		testFramework.AssertBranchDAG(t, map[string][]string{
			"TXA": {"MasterBranch"},
			"TXB": {"MasterBranch"},
			"TXC": {"MasterBranch"},
			"TXD": {"MasterBranch"},
			"TXH": {"MasterBranch"},
			"TXI": {"MasterBranch"},
			"TXF": {"TXC"},
			"TXG": {"TXC"},
		})

		assert.Equal(t, branchdag.Confirmed, testFramework.Ledger.BranchDAG.InclusionState(testFramework.BranchIDs("TXA")))
		assert.Equal(t, branchdag.Rejected, testFramework.Ledger.BranchDAG.InclusionState(testFramework.BranchIDs("TXB")))
		assert.Equal(t, branchdag.Pending, testFramework.Ledger.BranchDAG.InclusionState(testFramework.BranchIDs("TXC")))
		assert.Equal(t, branchdag.Pending, testFramework.Ledger.BranchDAG.InclusionState(testFramework.BranchIDs("TXD")))
		assert.Equal(t, branchdag.Pending, testFramework.Ledger.BranchDAG.InclusionState(testFramework.BranchIDs("TXH")))
		assert.Equal(t, branchdag.Pending, testFramework.Ledger.BranchDAG.InclusionState(testFramework.BranchIDs("TXI")))
		assert.Equal(t, branchdag.Pending, testFramework.Ledger.BranchDAG.InclusionState(testFramework.BranchIDs("TXF")))
		assert.Equal(t, branchdag.Pending, testFramework.Ledger.BranchDAG.InclusionState(testFramework.BranchIDs("TXG")))
	}

	require.True(t, testFramework.Ledger.BranchDAG.SetBranchConfirmed(branchdag.NewBranchID(testFramework.Transaction("TXD").ID())))

	// TX L combines a child (G) of a Rejected branch (C) and a pending branch H, resulting in (G,H)
	{
		for _, txAlias := range []string{"TXL"} {
			assert.NoError(t, testFramework.IssueTransaction(txAlias))
		}

		testFramework.AssertBranchIDs(t, map[string][]string{
			"G":   {"MasterBranch"},
			"TXA": {"TXA"},
			"TXB": {"TXB"},
			"TXC": {"TXC"},
			"TXD": {"TXD"},
			"TXH": {"TXH"},
			"TXI": {"TXI"},
			// Branches F & G are spawned by the fork of G
			"TXF": {"TXF"},
			"TXG": {"TXG"},
			"TXL": {"TXG", "TXH"},
		})

		testFramework.AssertBranchDAG(t, map[string][]string{
			"TXA": {"MasterBranch"},
			"TXB": {"MasterBranch"},
			"TXC": {"MasterBranch"},
			"TXD": {"MasterBranch"},
			"TXH": {"MasterBranch"},
			"TXI": {"MasterBranch"},
			"TXF": {"TXC"},
			"TXG": {"TXC"},
		})

		assert.Equal(t, branchdag.Confirmed, testFramework.Ledger.BranchDAG.InclusionState(testFramework.BranchIDs("TXA")))
		assert.Equal(t, branchdag.Rejected, testFramework.Ledger.BranchDAG.InclusionState(testFramework.BranchIDs("TXB")))
		assert.Equal(t, branchdag.Rejected, testFramework.Ledger.BranchDAG.InclusionState(testFramework.BranchIDs("TXC")))
		assert.Equal(t, branchdag.Confirmed, testFramework.Ledger.BranchDAG.InclusionState(testFramework.BranchIDs("TXD")))
		assert.Equal(t, branchdag.Pending, testFramework.Ledger.BranchDAG.InclusionState(testFramework.BranchIDs("TXH")))
		assert.Equal(t, branchdag.Pending, testFramework.Ledger.BranchDAG.InclusionState(testFramework.BranchIDs("TXI")))
		assert.Equal(t, branchdag.Rejected, testFramework.Ledger.BranchDAG.InclusionState(testFramework.BranchIDs("TXF")))
		assert.Equal(t, branchdag.Rejected, testFramework.Ledger.BranchDAG.InclusionState(testFramework.BranchIDs("TXG")))
		assert.Equal(t, branchdag.Rejected, testFramework.Ledger.BranchDAG.InclusionState(testFramework.BranchIDs("TXG", "TXH")))
	}

	require.True(t, testFramework.Ledger.BranchDAG.SetBranchConfirmed(branchdag.NewBranchID(testFramework.Transaction("TXH").ID())))

	// The new TX M should be now booked under G, as branch H confirmed, just G because we don't propagate H further.
	{
		for _, txAlias := range []string{"TXM"} {
			assert.NoError(t, testFramework.IssueTransaction(txAlias))
		}

		testFramework.AssertBranchIDs(t, map[string][]string{
			"G":   {"MasterBranch"},
			"TXA": {"TXA"},
			"TXB": {"TXB"},
			"TXC": {"TXC"},
			"TXD": {"TXD"},
			"TXH": {"TXH"},
			"TXI": {"TXI"},
			// Branches F & G are spawned by the fork of G
			"TXF": {"TXF"},
			"TXG": {"TXG"},
			"TXL": {"TXG", "TXH"},
			"TXM": {"TXG"},
		})

		testFramework.AssertBranchDAG(t, map[string][]string{
			"TXA": {"MasterBranch"},
			"TXB": {"MasterBranch"},
			"TXC": {"MasterBranch"},
			"TXD": {"MasterBranch"},
			"TXH": {"MasterBranch"},
			"TXI": {"MasterBranch"},
			"TXF": {"TXC"},
			"TXG": {"TXC"},
		})

		assert.Equal(t, branchdag.Confirmed, testFramework.Ledger.BranchDAG.InclusionState(testFramework.BranchIDs("TXA")))
		assert.Equal(t, branchdag.Rejected, testFramework.Ledger.BranchDAG.InclusionState(testFramework.BranchIDs("TXB")))
		assert.Equal(t, branchdag.Rejected, testFramework.Ledger.BranchDAG.InclusionState(testFramework.BranchIDs("TXC")))
		assert.Equal(t, branchdag.Confirmed, testFramework.Ledger.BranchDAG.InclusionState(testFramework.BranchIDs("TXD")))
		assert.Equal(t, branchdag.Confirmed, testFramework.Ledger.BranchDAG.InclusionState(testFramework.BranchIDs("TXH")))
		assert.Equal(t, branchdag.Rejected, testFramework.Ledger.BranchDAG.InclusionState(testFramework.BranchIDs("TXI")))
		assert.Equal(t, branchdag.Rejected, testFramework.Ledger.BranchDAG.InclusionState(testFramework.BranchIDs("TXF")))
		assert.Equal(t, branchdag.Rejected, testFramework.Ledger.BranchDAG.InclusionState(testFramework.BranchIDs("TXG")))
		assert.Equal(t, branchdag.Rejected, testFramework.Ledger.BranchDAG.InclusionState(testFramework.BranchIDs("TXG", "TXH")))
	}
}
