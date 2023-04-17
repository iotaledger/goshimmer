package realitiesledger_test

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/confirmation"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/realitiesledger"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/hive.go/runtime/workerpool"
)

func TestLedger_BookInOrder(t *testing.T) {
	workers := workerpool.NewGroup(t.Name())
	tf := realitiesledger.NewDefaultTestFramework(t, workers.CreateGroup("LedgerTestFramework"))

	tf.CreateTransaction("G", 3, "Genesis")
	tf.CreateTransaction("TX1", 1, "G.0")
	tf.CreateTransaction("TX1*", 1, "G.0")
	tf.CreateTransaction("TX2", 1, "G.1")
	tf.CreateTransaction("TX2*", 1, "G.1")
	tf.CreateTransaction("TX3", 1, "G.2")
	tf.CreateTransaction("TX3*", 1, "G.2")
	tf.CreateTransaction("TX4", 1, "TX1.0", "TX2.0")
	tf.CreateTransaction("TX5", 1, "TX4.0")
	tf.CreateTransaction("TX5*", 1, "TX4.0")
	tf.CreateTransaction("TX6", 1, "TX5.0", "TX3.0")
	tf.CreateTransaction("TX7", 1, "TX6.0")
	tf.CreateTransaction("TX7*", 1, "TX6.0")
	tf.CreateTransaction("TX8", 1, "TX7.0")

	{
		require.NoError(t, tf.IssueTransactions("G", "TX1", "TX1*", "TX2", "TX2*", "TX3", "TX3*", "TX4", "TX5", "TX6", "TX7", "TX8"))

		tf.AssertConflictIDs(map[string][]string{
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

		tf.AssertConflictDAG(map[string][]string{
			"TX1":  {},
			"TX1*": {},
			"TX2":  {},
			"TX2*": {},
			"TX3":  {},
			"TX3*": {},
		})
	}

	{
		require.NoError(t, tf.IssueTransactions("TX7*"))

		tf.AssertConflictIDs(map[string][]string{
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

		tf.AssertConflictDAG(map[string][]string{
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
		require.NoError(t, tf.IssueTransactions("TX5*"))

		tf.AssertConflictIDs(map[string][]string{
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

		tf.AssertConflictDAG(map[string][]string{
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

// See scenario at img/ledger_test_SetConflictConfirmed.png.
func TestLedger_SetConflictConfirmed(t *testing.T) {
	workers := workerpool.NewGroup(t.Name())
	tf := realitiesledger.NewDefaultTestFramework(t, workers.CreateGroup("LedgerTestFramework"))

	// Step 1: Bottom Layer
	tf.CreateTransaction("G", 3, "Genesis")
	tf.CreateTransaction("TXA", 1, "G.0")
	tf.CreateTransaction("TXB", 1, "G.0")
	tf.CreateTransaction("TXC", 1, "G.1")
	tf.CreateTransaction("TXD", 1, "G.1")
	tf.CreateTransaction("TXH", 1, "G.2")
	tf.CreateTransaction("TXI", 1, "G.2")
	// Step 2: Middle Layer
	tf.CreateTransaction("TXE", 1, "TXA.0", "TXC.0")
	// Step 3: Top Layer
	tf.CreateTransaction("TXF", 1, "TXE.0")
	// Step 4: Top Layer
	tf.CreateTransaction("TXG", 1, "TXE.0")
	// Step 5: TopTop Layer
	tf.CreateTransaction("TXL", 1, "TXG.0", "TXH.0")
	// Step 6: TopTopTOP Layer
	tf.CreateTransaction("TXM", 1, "TXL.0")

	// Mark A as Confirmed
	{
		require.NoError(t, tf.IssueTransactions("G", "TXA", "TXB", "TXC", "TXD", "TXH", "TXI"))
		require.True(t, tf.Instance.ConflictDAG().SetConflictAccepted(tf.Transaction("TXA").ID()))

		tf.AssertConflictIDs(map[string][]string{
			"G":   {},
			"TXA": {"TXA"},
			"TXB": {"TXB"},
			"TXC": {"TXC"},
			"TXD": {"TXD"},
			"TXH": {"TXH"},
			"TXI": {"TXI"},
		})

		tf.AssertConflictDAG(map[string][]string{
			"TXA": {},
			"TXB": {},
			"TXC": {},
			"TXD": {},
			"TXH": {},
			"TXI": {},
		})

		require.Equal(t, confirmation.Accepted, tf.Instance.ConflictDAG().ConfirmationState(tf.ConflictIDs("TXA")))
		require.Equal(t, confirmation.Rejected, tf.Instance.ConflictDAG().ConfirmationState(tf.ConflictIDs("TXB")))
		require.Equal(t, confirmation.Pending, tf.Instance.ConflictDAG().ConfirmationState(tf.ConflictIDs("TXC")))
		require.Equal(t, confirmation.Pending, tf.Instance.ConflictDAG().ConfirmationState(tf.ConflictIDs("TXD")))
		require.Equal(t, confirmation.Pending, tf.Instance.ConflictDAG().ConfirmationState(tf.ConflictIDs("TXH")))
		require.Equal(t, confirmation.Pending, tf.Instance.ConflictDAG().ConfirmationState(tf.ConflictIDs("TXI")))
	}

	// When creating the middle layer the new transaction E should be booked only under its Pending parent C
	{
		require.NoError(t, tf.IssueTransactions("TXE"))

		tf.AssertConflictIDs(map[string][]string{
			"G":   {},
			"TXA": {"TXA"},
			"TXB": {"TXB"},
			"TXC": {"TXC"},
			"TXD": {"TXD"},
			"TXH": {"TXH"},
			"TXI": {"TXI"},
			"TXE": {"TXC"},
		})

		tf.AssertConflictDAG(map[string][]string{
			"TXA": {},
			"TXB": {},
			"TXC": {},
			"TXD": {},
			"TXH": {},
			"TXI": {},
		})

		require.Equal(t, confirmation.Accepted, tf.Instance.ConflictDAG().ConfirmationState(tf.ConflictIDs("TXA")))
		require.Equal(t, confirmation.Rejected, tf.Instance.ConflictDAG().ConfirmationState(tf.ConflictIDs("TXB")))
		require.Equal(t, confirmation.Pending, tf.Instance.ConflictDAG().ConfirmationState(tf.ConflictIDs("TXC")))
		require.Equal(t, confirmation.Pending, tf.Instance.ConflictDAG().ConfirmationState(tf.ConflictIDs("TXD")))
		require.Equal(t, confirmation.Pending, tf.Instance.ConflictDAG().ConfirmationState(tf.ConflictIDs("TXH")))
		require.Equal(t, confirmation.Pending, tf.Instance.ConflictDAG().ConfirmationState(tf.ConflictIDs("TXI")))
	}

	// When creating the first transaction (F) of top layer it should be booked under the Pending parent C
	{
		require.NoError(t, tf.IssueTransactions("TXF"))

		tf.AssertConflictIDs(map[string][]string{
			"G":   {},
			"TXA": {"TXA"},
			"TXB": {"TXB"},
			"TXC": {"TXC"},
			"TXD": {"TXD"},
			"TXH": {"TXH"},
			"TXI": {"TXI"},
			// Conflicts F & G are spawned by the fork of G
			"TXF": {"TXC"},
		})

		tf.AssertConflictDAG(map[string][]string{
			"TXA": {},
			"TXB": {},
			"TXC": {},
			"TXD": {},
			"TXH": {},
			"TXI": {},
		})

		require.Equal(t, confirmation.Accepted, tf.Instance.ConflictDAG().ConfirmationState(tf.ConflictIDs("TXA")))
		require.Equal(t, confirmation.Rejected, tf.Instance.ConflictDAG().ConfirmationState(tf.ConflictIDs("TXB")))
		require.Equal(t, confirmation.Pending, tf.Instance.ConflictDAG().ConfirmationState(tf.ConflictIDs("TXC")))
		require.Equal(t, confirmation.Pending, tf.Instance.ConflictDAG().ConfirmationState(tf.ConflictIDs("TXD")))
		require.Equal(t, confirmation.Pending, tf.Instance.ConflictDAG().ConfirmationState(tf.ConflictIDs("TXH")))
		require.Equal(t, confirmation.Pending, tf.Instance.ConflictDAG().ConfirmationState(tf.ConflictIDs("TXI")))
	}

	// When creating the conflicting TX (G) of the top layer conflicts F & G are spawned by the fork of G
	{
		require.NoError(t, tf.IssueTransactions("TXG"))

		tf.AssertConflictIDs(map[string][]string{
			"G":   {},
			"TXA": {"TXA"},
			"TXB": {"TXB"},
			"TXC": {"TXC"},
			"TXD": {"TXD"},
			"TXH": {"TXH"},
			"TXI": {"TXI"},
			// Conflicts F & G are spawned by the fork of G
			"TXF": {"TXF"},
			"TXG": {"TXG"},
		})

		tf.AssertConflictDAG(map[string][]string{
			"TXA": {},
			"TXB": {},
			"TXC": {},
			"TXD": {},
			"TXH": {},
			"TXI": {},
			"TXF": {"TXC"},
			"TXG": {"TXC"},
		})

		require.Equal(t, confirmation.Accepted, tf.Instance.ConflictDAG().ConfirmationState(tf.ConflictIDs("TXA")))
		require.Equal(t, confirmation.Rejected, tf.Instance.ConflictDAG().ConfirmationState(tf.ConflictIDs("TXB")))
		require.Equal(t, confirmation.Pending, tf.Instance.ConflictDAG().ConfirmationState(tf.ConflictIDs("TXC")))
		require.Equal(t, confirmation.Pending, tf.Instance.ConflictDAG().ConfirmationState(tf.ConflictIDs("TXD")))
		require.Equal(t, confirmation.Pending, tf.Instance.ConflictDAG().ConfirmationState(tf.ConflictIDs("TXH")))
		require.Equal(t, confirmation.Pending, tf.Instance.ConflictDAG().ConfirmationState(tf.ConflictIDs("TXI")))
		require.Equal(t, confirmation.Pending, tf.Instance.ConflictDAG().ConfirmationState(tf.ConflictIDs("TXF")))
		require.Equal(t, confirmation.Pending, tf.Instance.ConflictDAG().ConfirmationState(tf.ConflictIDs("TXG")))
	}

	require.True(t, tf.Instance.ConflictDAG().SetConflictAccepted(tf.Transaction("TXD").ID()))

	// TX L combines a child (G) of a Rejected conflict (C) and a pending conflict H, resulting in (G,H)
	{
		require.NoError(t, tf.IssueTransactions("TXL"))

		tf.AssertConflictIDs(map[string][]string{
			"G":   {},
			"TXA": {"TXA"},
			"TXB": {"TXB"},
			"TXC": {"TXC"},
			"TXD": {"TXD"},
			"TXH": {"TXH"},
			"TXI": {"TXI"},
			// Conflicts F & G are spawned by the fork of G
			"TXF": {"TXF"},
			"TXG": {"TXG"},
			"TXL": {"TXG", "TXH"},
		})

		tf.AssertConflictDAG(map[string][]string{
			"TXA": {},
			"TXB": {},
			"TXC": {},
			"TXD": {},
			"TXH": {},
			"TXI": {},
			"TXF": {"TXC"},
			"TXG": {"TXC"},
		})

		require.Equal(t, confirmation.Accepted, tf.Instance.ConflictDAG().ConfirmationState(tf.ConflictIDs("TXA")))
		require.Equal(t, confirmation.Rejected, tf.Instance.ConflictDAG().ConfirmationState(tf.ConflictIDs("TXC")))
		require.Equal(t, confirmation.Accepted, tf.Instance.ConflictDAG().ConfirmationState(tf.ConflictIDs("TXD")))
		require.Equal(t, confirmation.Pending, tf.Instance.ConflictDAG().ConfirmationState(tf.ConflictIDs("TXH")))
		require.Equal(t, confirmation.Pending, tf.Instance.ConflictDAG().ConfirmationState(tf.ConflictIDs("TXI")))
		require.Equal(t, confirmation.Rejected, tf.Instance.ConflictDAG().ConfirmationState(tf.ConflictIDs("TXF")))
		require.Equal(t, confirmation.Rejected, tf.Instance.ConflictDAG().ConfirmationState(tf.ConflictIDs("TXG")))
		require.Equal(t, confirmation.Rejected, tf.Instance.ConflictDAG().ConfirmationState(tf.ConflictIDs("TXG", "TXH")))
	}

	require.True(t, tf.Instance.ConflictDAG().SetConflictAccepted(tf.Transaction("TXH").ID()))

	// The new TX M should be now booked under G, as conflict H confirmed, just G because we don't propagate H further.
	{
		require.NoError(t, tf.IssueTransactions("TXM"))

		tf.AssertConflictIDs(map[string][]string{
			"G":   {},
			"TXA": {"TXA"},
			"TXB": {"TXB"},
			"TXC": {"TXC"},
			"TXD": {"TXD"},
			"TXH": {"TXH"},
			"TXI": {"TXI"},
			// Conflicts F & G are spawned by the fork of G
			"TXF": {"TXF"},
			"TXG": {"TXG"},
			"TXL": {"TXG", "TXH"},
			"TXM": {"TXG"},
		})

		tf.AssertConflictDAG(map[string][]string{
			"TXA": {},
			"TXB": {},
			"TXC": {},
			"TXD": {},
			"TXH": {},
			"TXI": {},
			"TXF": {"TXC"},
			"TXG": {"TXC"},
		})

		require.Equal(t, confirmation.Accepted, tf.Instance.ConflictDAG().ConfirmationState(tf.ConflictIDs("TXA")))
		require.Equal(t, confirmation.Rejected, tf.Instance.ConflictDAG().ConfirmationState(tf.ConflictIDs("TXB")))
		require.Equal(t, confirmation.Rejected, tf.Instance.ConflictDAG().ConfirmationState(tf.ConflictIDs("TXC")))
		require.Equal(t, confirmation.Accepted, tf.Instance.ConflictDAG().ConfirmationState(tf.ConflictIDs("TXD")))
		require.Equal(t, confirmation.Accepted, tf.Instance.ConflictDAG().ConfirmationState(tf.ConflictIDs("TXH")))
		require.Equal(t, confirmation.Rejected, tf.Instance.ConflictDAG().ConfirmationState(tf.ConflictIDs("TXI")))
		require.Equal(t, confirmation.Rejected, tf.Instance.ConflictDAG().ConfirmationState(tf.ConflictIDs("TXF")))
		require.Equal(t, confirmation.Rejected, tf.Instance.ConflictDAG().ConfirmationState(tf.ConflictIDs("TXG")))
		require.Equal(t, confirmation.Rejected, tf.Instance.ConflictDAG().ConfirmationState(tf.ConflictIDs("TXG", "TXH")))
	}
}

func TestLedger_SolidifyAndForkMultiThreaded(t *testing.T) {
	workers := workerpool.NewGroup(t.Name())
	tf := realitiesledger.NewDefaultTestFramework(t, workers.CreateGroup("LedgerTestFramework"))

	createdAliases := make([]string, 0)
	bookedAliases := make(map[string]bool)

	// Create UTXO-DAG from transactions 100 layers deep.
	{
		tf.CreateTransaction("G", 10, "Genesis")
		require.NoError(t, tf.IssueTransactions("G"))

		for layer := 0; layer < 100; layer++ {
			for txNum := 0; txNum < 10; txNum++ {
				txAlias := fmt.Sprintf("TX-%d-%d", layer, txNum)
				createdAliases = append(createdAliases, txAlias)
				consumedAliases := make([]string, 0)

				if layer == 0 {
					// We need to treat the bottom layer differently since it consumes from Genesis.
					consumedAliases = append(consumedAliases, fmt.Sprintf("G.%d", txNum))

					// Fork bottom layer.
					genesisTxAlias := fmt.Sprintf("TX-0-%d*", txNum)
					createdAliases = append(createdAliases, genesisTxAlias)
					tf.CreateTransaction(genesisTxAlias, 10, consumedAliases...)

					bookedAliases[genesisTxAlias] = true
				} else {
					for inputConsumed := 0; inputConsumed < 10; inputConsumed++ {
						consumedAliases = append(consumedAliases, fmt.Sprintf("TX-%d-%d.%d", layer-1, txNum, inputConsumed))
					}
				}

				tf.CreateTransaction(txAlias, 10, consumedAliases...)
				bookedAliases[txAlias] = true
			}
		}
	}

	// Shuffle and issue all transactions created so far concurrently.
	{
		rand.Shuffle(len(createdAliases), func(i, j int) {
			createdAliases[i], createdAliases[j] = createdAliases[j], createdAliases[i]
		})

		var wg sync.WaitGroup
		for _, createdAlias := range createdAliases {
			wg.Add(1)
			go func(createdAlias string) {
				defer wg.Done()
				err := tf.IssueTransactions(createdAlias)
				if err != nil && !errors.Is(err, mempool.ErrTransactionUnsolid) {
					panic(err)
				}
			}(createdAlias)
		}
		wg.Wait()
	}

	// Create ad-hoc TX11 to mix and match conflicts propagated from the bottom layer.
	{
		tf.CreateTransaction("TX11", 10, "TX-99-0.0", "TX-99-1.0", "TX-99-2.0", "TX-99-3.0", "TX-99-4.0", "TX-99-5.0", "TX-99-6.0", "TX-99-7.0", "TX-99-8.0", "TX-99-9.0")
		tf.IssueTransactions("TX11")
	}

	// Verify conflicts for all transactions in the layers.
	{
		workers.WaitChildren()
		tf.AssertBooked(bookedAliases)

		mappedConflicts := make(map[string][]string)
		for _, issuedAlias := range createdAliases {
			conflictNum := strings.Split(issuedAlias, "-")[2]
			mappedConflicts[issuedAlias] = []string{fmt.Sprintf("TX-0-%s", conflictNum)}
		}
		tf.AssertConflictIDs(mappedConflicts)
	}

	// Verify conflicts for TX11.
	{
		tf.AssertConflictIDs(map[string][]string{
			"TX11": {"TX-0-0", "TX-0-1", "TX-0-2", "TX-0-3", "TX-0-4", "TX-0-5", "TX-0-6", "TX-0-7", "TX-0-8", "TX-0-9"},
		})
	}

	// Create TX12 with deeper double spends.
	{
		tf.CreateTransaction("TX12", 10, "TX-0-0.0", "TX-0-1.0", "TX-0-2.0", "TX-0-3.0", "TX-0-4.0", "TX-0-5.0", "TX-0-6.0", "TX-0-7.0", "TX-0-8.0", "TX-0-9.0")
		require.NoError(t, tf.IssueTransactions("TX12"))

		workers.WaitChildren()

		tf.AssertConflictIDs(map[string][]string{
			"TX11": {"TX-1-0", "TX-1-1", "TX-1-2", "TX-1-3", "TX-1-4", "TX-1-5", "TX-1-6", "TX-1-7", "TX-1-8", "TX-1-9"},
			"TX12": {"TX12"},
		})

		tf.AssertConflictDAG(map[string][]string{
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

		mappedConflicts := make(map[string][]string)
		for _, issuedAlias := range createdAliases {
			layer := strings.Split(issuedAlias, "-")[1]
			conflictNum := strings.Split(issuedAlias, "-")[2]

			var conflict string
			if layer == "0" {
				conflict = fmt.Sprintf("TX-0-%s", conflictNum)
			} else {
				conflict = fmt.Sprintf("TX-1-%s", conflictNum)
			}
			mappedConflicts[issuedAlias] = []string{conflict}
		}
		tf.AssertConflictIDs(mappedConflicts)
	}
}

func TestLedger_ForkAlreadyConflictingTransaction(t *testing.T) {
	workers := workerpool.NewGroup(t.Name())
	tf := realitiesledger.NewDefaultTestFramework(t, workers.CreateGroup("LedgerTestFramework"))

	tf.CreateTransaction("G", 3, "Genesis")
	tf.CreateTransaction("TX1", 1, "G.0", "G.1")
	tf.CreateTransaction("TX2", 1, "G.0")
	tf.CreateTransaction("TX3", 1, "G.1")

	require.NoError(t, tf.IssueTransactions("G", "TX1", "TX2", "TX3"))

	tf.AssertConflictIDs(map[string][]string{
		"G":   {},
		"TX1": {"TX1"},
		"TX2": {"TX2"},
		"TX3": {"TX3"},
	})

	tf.AssertConflictDAG(map[string][]string{
		"TX1": {},
		"TX2": {},
		"TX3": {},
	})

	tf.AssertConflicts(map[string][]string{
		"G.0": {"TX1", "TX2"},
		"G.1": {"TX1", "TX3"},
	})
}

func TestLedger_TransactionCausallyRelated(t *testing.T) {
	workers := workerpool.NewGroup(t.Name())
	tf := realitiesledger.NewDefaultTestFramework(t, workers.CreateGroup("LedgerTestFramework"))

	tf.CreateTransaction("G", 3, "Genesis")
	tf.CreateTransaction("TX1", 1, "G.0")
	tf.CreateTransaction("TX2", 1, "TX1.0")
	tf.CreateTransaction("TX3", 1, "G.0", "TX2.0")

	require.NoError(t, tf.IssueTransactions("G", "TX1", "TX2"))
	require.EqualError(t, tf.IssueTransactions("TX3"), "failed to issue transaction 'TX3': TransactionID(TX3) is trying to spend causally related Outputs: transaction invalid")
}

func TestLedger_Aliases(t *testing.T) {
	var transactionID utxo.TransactionID
	require.NoError(t, transactionID.FromRandomness())

	conflictID := transactionID

	transactionID.RegisterAlias("TX1")
	conflictID.RegisterAlias("Conflict1")
}

func TestLedger_AcceptanceRaceCondition(t *testing.T) {
	workers := workerpool.NewGroup(t.Name())
	defer workers.Shutdown()

	tf := realitiesledger.NewDefaultTestFramework(t, workers.CreateGroup("LedgerTestFramework"))

	tf.CreateTransaction("TX1", 1, "Genesis")
	tf.CreateTransaction("TX1*", 1, "Genesis")
	tf.CreateTransaction("TX2", 1, "TX1.0")

	require.NoError(t, tf.IssueTransactions("TX1", "TX2"))
	tf.Instance.SetTransactionInclusionSlot(tf.Transaction("TX1").ID(), 1)

	tf.AssertTransactionConfirmationState("TX1", confirmation.State.IsAccepted)

	require.NoError(t, tf.IssueTransactions("TX1*"))

	tf.AssertTransactionConfirmationState("TX1", confirmation.State.IsAccepted)
	tf.AssertBranchConfirmationState("TX1", confirmation.State.IsAccepted)

	tf.AssertTransactionConfirmationState("TX1*", confirmation.State.IsRejected)
	tf.AssertBranchConfirmationState("TX1*", confirmation.State.IsRejected)

	tf.AssertConflictIDs(map[string][]string{
		"TX1":  {},
		"TX2":  {},
		"TX1*": {"TX1*"},
	})

	tf.AssertConflictDAG(map[string][]string{
		"TX1":  {},
		"TX1*": {},
	})

	tf.AssertConflicts(map[string][]string{
		"Genesis": {"TX1", "TX1*"},
	})
}
