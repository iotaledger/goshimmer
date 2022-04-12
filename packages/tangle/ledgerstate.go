package tangle

// region LedgerstateOLD //////////////////////////////////////////////////////////////////////////////////////////////////

// LedgerstateOLD is a Tangle component that wraps the components of the ledger package and makes them available at a
// "single point of contact".
type LedgerstateOLD struct {
	tangle      *Tangle
	totalSupply uint64
}

// NewLedger is the constructor of the LedgerstateOLD component.
func NewLedger(tangle *Tangle) (ledgerState *LedgerstateOLD) {
	return &LedgerstateOLD{
		tangle: tangle,
	}
}

// LoadSnapshot creates a set of outputs in the UTXO-DAG, that are forming the genesis for future transactions.
// func (l *LedgerstateOLD) LoadSnapshot(snapshot *ledgerstate.Snapshot) (err error) {
// 	l.UTXODAG.LoadSnapshot(snapshot)
// 	// add attachment link between txs from snapshot and the genesis message (EmptyMessageID).
// 	for txID, record := range snapshot.Transactions {
// 		attachment, _ := l.tangle.Storage.StoreAttachment(txID, EmptyMessageID)
// 		if attachment != nil {
// 			attachment.Release()
// 		}
// 		for i, output := range record.Essence.Outputs() {
// 			if !record.UnspentOutputs[i] {
// 				continue
// 			}
// 			output.Balances().ForEach(func(color ledgerstate.Color, balance uint64) bool {
// 				l.totalSupply += balance
// 				return true
// 			})
// 		}
// 	}
// 	attachment, _ := l.tangle.Storage.StoreAttachment(ledgerstate.GenesisTransactionID, EmptyMessageID)
// 	if attachment != nil {
// 		attachment.Release()
// 	}
// 	return
// }
//
// // SnapshotUTXO returns the UTXO snapshot, which is a list of transactions with unspent outputs.
// func (l *LedgerstateOLD) SnapshotUTXO() (snapshot *ledgerstate.Snapshot) {
// 	// The following parameter should be larger than the max allowed timestamp variation, and the required time for confirmation.
// 	// We can snapshot this far in the past, since global snapshots don't occur frequent, and it is ok to ignore the last few minutes.
// 	minAge := 120 * time.Second
// 	snapshot = &ledgerstate.Snapshot{
// 		Transactions: make(map[ledgerstate.TransactionID]ledgerstate.Record),
// 	}
//
// 	startSnapshot := time.Now()
// 	copyLedgerState := l.Transactions() // consider that this may take quite some time
//
// 	for _, transaction := range copyLedgerState {
// 		// skip transactions that are not confirmed
// 		var isUnconfirmed bool
// 		l.TransactionMetadata(transaction.ID()).Consume(func(transactionMetadata *ledgerstate.TransactionMetadata) {
// 			for branchID := range transactionMetadata.BranchIDs() {
// 				if !l.tangle.ConfirmationOracle.IsBranchConfirmed(branchID) {
// 					isUnconfirmed = true
// 					break
// 				}
// 			}
// 		})
//
// 		if isUnconfirmed {
// 			continue
// 		}
//
// 		// skip transactions that are too recent before startSnapshot
// 		if startSnapshot.Sub(transaction.Essence().Timestamp()) < minAge {
// 			continue
// 		}
// 		unspentOutputs := make([]bool, len(transaction.Essence().Outputs()))
// 		includeTransaction := false
// 		for i, output := range transaction.Essence().Outputs() {
// 			unspentOutputs[i] = true
// 			includeTransaction = true
//
// 			confirmedConsumerID := l.ConfirmedConsumer(output.ID())
// 			if confirmedConsumerID != ledgerstate.GenesisTransactionID {
// 				tx, exists := copyLedgerState[confirmedConsumerID]
// 				// If the Confirmed Consumer is old enough we consider the output spent
// 				if exists && startSnapshot.Sub(tx.Essence().Timestamp()) >= minAge {
// 					unspentOutputs[i] = false
// 					includeTransaction = false
// 				}
// 			}
// 		}
// 		// include only transactions with at least one unspent output
// 		if includeTransaction {
// 			snapshot.Transactions[transaction.ID()] = ledgerstate.Record{
// 				Essence:        transaction.Essence(),
// 				UnlockBlocks:   transaction.UnlockBlocks(),
// 				UnspentOutputs: unspentOutputs,
// 			}
// 		}
// 	}
//
// 	// TODO ??? due to possible race conditions we could add a check for the consistency of the UTXO snapshot
//
// 	return snapshot
// }
//
// // Transactions returns all the transactions.
// func (l *LedgerstateOLD) Transactions() (transactions map[ledgerstate.TransactionID]*ledgerstate.Transaction) {
// 	return l.UTXODAG.Transactions()
// }
//
// // CachedOutputsOnAddress retrieves all the Outputs that are associated with an address.
// func (l *LedgerstateOLD) CachedOutputsOnAddress(address ledgerstate.Address) (cachedOutputs objectstorage.CachedObjects[ledgerstate.Output]) {
// 	l.UTXODAG.CachedAddressOutputMapping(address).Consume(func(addressOutputMapping *ledgerstate.AddressOutputMapping) {
// 		cachedOutputs = append(cachedOutputs, l.CachedOutput(addressOutputMapping.OutputID()))
// 	})
// 	return
// }

// TotalSupply returns the total supply.
func (l *LedgerstateOLD) TotalSupply() (totalSupply uint64) {
	return l.totalSupply
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
