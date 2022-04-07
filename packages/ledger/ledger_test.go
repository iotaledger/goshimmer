package ledger

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/ledger/branchdag"
)

func TestLedger_BookInOrder(t *testing.T) {
	testFramework := NewTestFramework(t)
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

		testFramework.AssertBranchIDs(map[string][]string{
			"G":    {},
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

		testFramework.AssertBranchDAG(map[string][]string{
			"TX1":  {},
			"TX1*": {},
			"TX2":  {},
			"TX2*": {},
			"TX3":  {},
			"TX3*": {},
		})
	}

	{
		assert.NoError(t, testFramework.IssueTransaction("TX7*"))

		testFramework.AssertBranchIDs(map[string][]string{
			"G":    {},
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

		testFramework.AssertBranchDAG(map[string][]string{
			"TX1":  {},
			"TX1*": {},
			"TX2":  {},
			"TX2*": {},
			"TX3":  {},
			"TX3*": {},
			"TX7":  {"TX1", "TX2", "TX3"},
			"TX7*": {"TX1", "TX2", "TX3"},
		})
	}

	{
		assert.NoError(t, testFramework.IssueTransaction("TX5*"))

		testFramework.AssertBranchIDs(map[string][]string{
			"G":    {},
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

		testFramework.AssertBranchDAG(map[string][]string{
			"TX1":  {},
			"TX1*": {},
			"TX2":  {},
			"TX2*": {},
			"TX3":  {},
			"TX3*": {},
			"TX5":  {"TX1", "TX2"},
			"TX5*": {"TX1", "TX2"},
			"TX7":  {"TX5", "TX3"},
			"TX7*": {"TX5", "TX3"},
		})
	}
}

// See scenario at img/ledger_test_SetBranchConfirmed.png
func TestLedger_SetBranchConfirmed(t *testing.T) {
	testFramework := NewTestFramework(t)

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

		testFramework.AssertBranchIDs(map[string][]string{
			"G":   {},
			"TXA": {"TXA"},
			"TXB": {"TXB"},
			"TXC": {"TXC"},
			"TXD": {"TXD"},
			"TXH": {"TXH"},
			"TXI": {"TXI"},
		})

		testFramework.AssertBranchDAG(map[string][]string{
			"TXA": {},
			"TXB": {},
			"TXC": {},
			"TXD": {},
			"TXH": {},
			"TXI": {},
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

		testFramework.AssertBranchIDs(map[string][]string{
			"G":   {},
			"TXA": {"TXA"},
			"TXB": {"TXB"},
			"TXC": {"TXC"},
			"TXD": {"TXD"},
			"TXH": {"TXH"},
			"TXI": {"TXI"},
			"TXE": {"TXC"},
		})

		testFramework.AssertBranchDAG(map[string][]string{
			"TXA": {},
			"TXB": {},
			"TXC": {},
			"TXD": {},
			"TXH": {},
			"TXI": {},
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

		testFramework.AssertBranchIDs(map[string][]string{
			"G":   {},
			"TXA": {"TXA"},
			"TXB": {"TXB"},
			"TXC": {"TXC"},
			"TXD": {"TXD"},
			"TXH": {"TXH"},
			"TXI": {"TXI"},
			// Branches F & G are spawned by the fork of G
			"TXF": {"TXC"},
		})

		testFramework.AssertBranchDAG(map[string][]string{
			"TXA": {},
			"TXB": {},
			"TXC": {},
			"TXD": {},
			"TXH": {},
			"TXI": {},
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

		testFramework.AssertBranchIDs(map[string][]string{
			"G":   {},
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

		testFramework.AssertBranchDAG(map[string][]string{
			"TXA": {},
			"TXB": {},
			"TXC": {},
			"TXD": {},
			"TXH": {},
			"TXI": {},
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

		testFramework.AssertBranchIDs(map[string][]string{
			"G":   {},
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

		testFramework.AssertBranchDAG(map[string][]string{
			"TXA": {},
			"TXB": {},
			"TXC": {},
			"TXD": {},
			"TXH": {},
			"TXI": {},
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

		testFramework.AssertBranchIDs(map[string][]string{
			"G":   {},
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

		testFramework.AssertBranchDAG(map[string][]string{
			"TXA": {},
			"TXB": {},
			"TXC": {},
			"TXD": {},
			"TXH": {},
			"TXI": {},
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

func TestLedger_SolidifyAndForkMultiThreaded(t *testing.T) {
	testFramework := NewTestFramework(t)

	createdAliases := make([]string, 0)
	bookedAliases := make(map[string]bool)

	// Create UTXO-DAG from transactions 100 layers deep.
	{
		testFramework.CreateTransaction("G", 10, "Genesis")
		assert.NoError(t, testFramework.IssueTransaction("G"))

		for layer := 0; layer < 100; layer++ {
			for txNum := 0; txNum < 10; txNum++ {
				txAlias := fmt.Sprintf("TX-%d-%d", layer, txNum)
				createdAliases = append(createdAliases, txAlias)
				consumedAliases := make([]string, 0)

				if layer == 0 {
					// We need to treat the bottom layer differently since it consumes from Genesis.
					consumedAliases = append(consumedAliases, fmt.Sprintf("G.%d", txNum))

					// Fork bottom layer.
					txAlias := fmt.Sprintf("TX-0-%d*", txNum)
					createdAliases = append(createdAliases, txAlias)
					testFramework.CreateTransaction(txAlias, 10, consumedAliases...)

					bookedAliases[txAlias] = true
				} else {
					for inputConsumed := 0; inputConsumed < 10; inputConsumed++ {
						consumedAliases = append(consumedAliases, fmt.Sprintf("TX-%d-%d.%d", layer-1, txNum, inputConsumed))
					}
				}

				testFramework.CreateTransaction(txAlias, 10, consumedAliases...)
				bookedAliases[txAlias] = true
			}
		}
	}

	// Shuffle and issue all transactions created so far concurrently.
	{
		rand.Shuffle(len(createdAliases), func(i, j int) {
			createdAliases[i], createdAliases[j] = createdAliases[j], createdAliases[i]
		})
		for _, createdAlias := range createdAliases {
			go func(createdAlias string) {
				err := testFramework.IssueTransaction(createdAlias)
				if err != nil && !errors.Is(err, ErrTransactionUnsolid) {
					panic(err)
				}
			}(createdAlias)
		}
	}

	// Create ad-hoc TX11 to mix and match branches propagated from the bottom layer.
	{
		testFramework.CreateTransaction("TX11", 10, "TX-99-0.0", "TX-99-1.0", "TX-99-2.0", "TX-99-3.0", "TX-99-4.0", "TX-99-5.0", "TX-99-6.0", "TX-99-7.0", "TX-99-8.0", "TX-99-9.0")
		assert.ErrorIs(t, testFramework.IssueTransaction("TX11"), ErrTransactionUnsolid)
	}

	// Verify branches for all transactions in the layers.
	{
		assert.Eventually(t, func() bool {
			return testFramework.AllBooked(createdAliases...)
		}, 5*time.Second, 100*time.Millisecond)
		testFramework.AssertBooked(bookedAliases)

		mappedBranches := make(map[string][]string)
		for _, issuedAlias := range createdAliases {
			branchNum := strings.Split(issuedAlias, "-")[2]
			mappedBranches[issuedAlias] = []string{fmt.Sprintf("TX-0-%s", branchNum)}
		}
		testFramework.AssertBranchIDs(mappedBranches)
	}

	// Verify branches for TX11.
	{
		assert.Eventually(t, func() bool {
			return testFramework.AllBooked("TX11")
		}, 5*time.Second, 100*time.Millisecond)

		testFramework.AssertBranchIDs(map[string][]string{
			"TX11": {"TX-0-0", "TX-0-1", "TX-0-2", "TX-0-3", "TX-0-4", "TX-0-5", "TX-0-6", "TX-0-7", "TX-0-8", "TX-0-9"},
		})
	}

	// Create TX12 with deeper double spends.
	{
		testFramework.CreateTransaction("TX12", 10, "TX-0-0.0", "TX-0-1.0", "TX-0-2.0", "TX-0-3.0", "TX-0-4.0", "TX-0-5.0", "TX-0-6.0", "TX-0-7.0", "TX-0-8.0", "TX-0-9.0")
		assert.NoError(t, testFramework.IssueTransaction("TX12"))

		testFramework.AssertBranchIDs(map[string][]string{
			"TX11": {"TX-1-0", "TX-1-1", "TX-1-2", "TX-1-3", "TX-1-4", "TX-1-5", "TX-1-6", "TX-1-7", "TX-1-8", "TX-1-9"},
			"TX12": {"TX12"},
		})

		testFramework.AssertBranchDAG(map[string][]string{
			"TX-0-0": {},
			"TX-0-1": {},
			"TX-0-2": {},
			"TX-0-3": {},
			"TX-0-4": {},
			"TX-0-5": {},
			"TX-0-6": {},
			"TX-0-7": {},
			"TX-0-8": {},
			"TX-0-9": {},
			"TX-1-0": {"TX-0-0"},
			"TX-1-1": {"TX-0-1"},
			"TX-1-2": {"TX-0-2"},
			"TX-1-3": {"TX-0-3"},
			"TX-1-4": {"TX-0-4"},
			"TX-1-5": {"TX-0-5"},
			"TX-1-6": {"TX-0-6"},
			"TX-1-7": {"TX-0-7"},
			"TX-1-8": {"TX-0-8"},
			"TX-1-9": {"TX-0-9"},
			"TX12":   {"TX-0-0", "TX-0-1", "TX-0-2", "TX-0-3", "TX-0-4", "TX-0-5", "TX-0-6", "TX-0-7", "TX-0-8", "TX-0-9"},
		})

		mappedBranches := make(map[string][]string)
		for _, issuedAlias := range createdAliases {
			layer := strings.Split(issuedAlias, "-")[1]
			branchNum := strings.Split(issuedAlias, "-")[2]

			var branch string
			if layer == "0" {
				branch = fmt.Sprintf("TX-0-%s", branchNum)
			} else {
				branch = fmt.Sprintf("TX-1-%s", branchNum)
			}
			mappedBranches[issuedAlias] = []string{branch}
		}
		testFramework.AssertBranchIDs(mappedBranches)
	}
}

func TestLedger_ForkAlreadyConflictingTransaction(t *testing.T) {
	testFramework := NewTestFramework(t)

	testFramework.CreateTransaction("G", 3, "Genesis")
	testFramework.CreateTransaction("TX1", 1, "G.0", "G.1")
	testFramework.CreateTransaction("TX2", 1, "G.0")
	testFramework.CreateTransaction("TX3", 1, "G.1")

	for _, txAlias := range []string{"G", "TX1", "TX2", "TX3"} {
		assert.NoError(t, testFramework.IssueTransaction(txAlias))
	}

	testFramework.AssertBranchIDs(map[string][]string{
		"G":   {},
		"TX1": {"TX1"},
		"TX2": {"TX2"},
		"TX3": {"TX3"},
	})

	testFramework.AssertBranchDAG(map[string][]string{
		"TX1": {},
		"TX2": {},
		"TX3": {},
	})

	testFramework.AssertConflicts(map[string][]string{
		"G.0": {"TX1", "TX2"},
		"G.1": {"TX1", "TX3"},
	})
}

func TestLedger_TransactionCausallyRelated(t *testing.T) {
	testFramework := NewTestFramework(t)

	testFramework.CreateTransaction("G", 3, "Genesis")
	testFramework.CreateTransaction("TX1", 1, "G.0")
	testFramework.CreateTransaction("TX2", 1, "TX1.0")
	testFramework.CreateTransaction("TX3", 1, "G.0", "TX2.0")

	for _, txAlias := range []string{"G", "TX1", "TX2"} {
		assert.NoError(t, testFramework.IssueTransaction(txAlias))
	}

	assert.EqualError(t, testFramework.IssueTransaction("TX3"), "TransactionID(TX3) is trying to spend causally related Outputs: transaction invalid")
}
