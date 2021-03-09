package utxodb

import (
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"golang.org/x/xerrors"
)

// IsConfirmed checks if the transaction is in the UTXODB (in the ledger)
func (u *UtxoDB) IsConfirmed(txid *ledgerstate.TransactionID) bool {
	u.mutex.Lock()
	defer u.mutex.Unlock()
	_, ok := u.transactions[*txid]
	return ok
}

// AddTransaction adds transaction to UTXODB or return an error.
// The function ensures consistency of the UTXODB ledger
func (u *UtxoDB) AddTransaction(tx *ledgerstate.Transaction) error {
	u.mutex.Lock()
	defer u.mutex.Unlock()

	if err := u.checkTransaction(tx); err != nil {
		return err
	}
	// delete consumed (referenced) outputs from the ledger
	for _, inp := range tx.Essence().Inputs() {
		utxoInp := inp.(*ledgerstate.UTXOInput)
		delete(u.utxo, utxoInp.ReferencedOutputID())
	}
	// add outputs to the ledger
	for _, out := range tx.Essence().Outputs() {
		if out.ID().TransactionID() != tx.ID() {
			panic("utxodb.AddTransaction: incorrect output ID")
		}
		var outClone ledgerstate.Output
		switch o := out.(type) {
		case *ledgerstate.SigLockedColoredOutput:
			outClone = o.UpdateMintingColor()
		case *ledgerstate.SigLockedSingleOutput:
			outClone = out.Clone()
		default:
			panic("utxodb.AddTransaction: unknown type")
		}
		u.utxo[out.ID()] = outClone
	}
	u.transactions[tx.ID()] = tx
	u.checkLedgerBalance()
	return nil
}

// GetTransaction retrieves value transaction by its hash (ID)
func (u *UtxoDB) GetTransaction(id ledgerstate.TransactionID) (*ledgerstate.Transaction, bool) {
	u.mutex.RLock()
	defer u.mutex.RUnlock()

	return u.getTransaction(id)
}

// MustGetTransaction same as GetTransaction only panics if transaction is not in UTXODB
func (u *UtxoDB) MustGetTransaction(id ledgerstate.TransactionID) *ledgerstate.Transaction {
	u.mutex.RLock()
	defer u.mutex.RUnlock()
	return u.mustGetTransaction(id)
}

// GetAddressOutputs returns outputs contained in the address
func (u *UtxoDB) GetAddressOutputs(addr ledgerstate.Address) []ledgerstate.Output {
	u.mutex.RLock()
	defer u.mutex.RUnlock()

	return u.getAddressOutputs(addr)
}

func (u *UtxoDB) GetAddressBalances(addr ledgerstate.Address) map[ledgerstate.Color]uint64 {
	ret := make(map[ledgerstate.Color]uint64)
	outputs := u.GetAddressOutputs(addr)
	for _, out := range outputs {
		out.Balances().ForEach(func(col ledgerstate.Color, bal uint64) bool {
			s, _ := ret[col]
			ret[col] = s + bal
			return true
		})
	}
	return ret
}

func (u *UtxoDB) Balance(addr ledgerstate.Address, color ledgerstate.Color) uint64 {
	bals := u.GetAddressBalances(addr)
	ret, _ := bals[color]
	return ret
}

func (u *UtxoDB) BalanceIOTA(addr ledgerstate.Address) uint64 {
	return u.Balance(addr, ledgerstate.ColorIOTA)
}

func (u *UtxoDB) getTransaction(id ledgerstate.TransactionID) (*ledgerstate.Transaction, bool) {
	tx, ok := u.transactions[id]
	return tx, ok
}

func (u *UtxoDB) mustGetTransaction(id ledgerstate.TransactionID) *ledgerstate.Transaction {
	tx, ok := u.transactions[id]
	if !ok {
		panic(xerrors.Errorf("utxodb.mustGetTransaction: tx id doesn't exist: %s", id.String()))
	}
	return tx
}

func (u *UtxoDB) getAddressOutputs(addr ledgerstate.Address) []ledgerstate.Output {
	addrArr := addr.Array()
	ret := make([]ledgerstate.Output, 0)
	for _, out := range u.utxo {
		if out.Address().Array() == addrArr {
			ret = append(ret, out)
		}
	}
	return ret
}

func (u *UtxoDB) getOutputTotal(outid ledgerstate.OutputID) (uint64, error) {
	out, ok := u.utxo[outid]
	if !ok {
		return 0, xerrors.Errorf("no such output: %s", outid.String())
	}
	ret := uint64(0)
	out.Balances().ForEach(func(_ ledgerstate.Color, bal uint64) bool {
		ret += bal
		return true
	})
	return ret, nil
}

func (u *UtxoDB) checkLedgerBalance() {
	total := uint64(0)
	for outp := range u.utxo {
		b, err := u.getOutputTotal(outp)
		if err != nil {
			panic("utxodb: wrong ledger balance: " + err.Error())
		}
		total += b
	}
	if total != Supply {
		panic("utxodb: wrong ledger balance")
	}
}

func (u *UtxoDB) collectConsumedOutputs(tx *ledgerstate.Transaction) ([]ledgerstate.Output, error) {
	ret := make([]ledgerstate.Output, len(tx.Essence().Inputs()))
	for i, inp := range tx.Essence().Inputs() {
		if inp.Type() != ledgerstate.UTXOInputType {
			return nil, xerrors.New("utxodb.collectInputBalances: wrong input type")
		}
		utxoInp := inp.(*ledgerstate.UTXOInput)
		var ok bool
		oid := utxoInp.ReferencedOutputID()
		if ret[i], ok = u.utxo[oid]; !ok {
			return nil, xerrors.New("utxodb.collectInputBalances: output does not exist")
		}
		otx, ok := u.getTransaction(oid.TransactionID())
		if !ok {
			return nil, xerrors.Errorf("input transaction not found: %s", oid.TransactionID())
		}
		if tx.Essence().Timestamp().Before(otx.Essence().Timestamp()) {
			return nil, xerrors.Errorf("transaction timestamp is before input timestamp: %s", oid.TransactionID())
		}
	}
	return ret, nil
}

// checkTransaction checks the same way as ledgerstate
func (u *UtxoDB) checkTransaction(tx *ledgerstate.Transaction) error {
	inputs, err := u.collectConsumedOutputs(tx)
	if err != nil {
		return err
	}
	if !ledgerstate.TransactionBalancesValid(inputs, tx.Essence().Outputs()) {
		return xerrors.Errorf("sum of consumed and spent balances is not 0")
	}
	if !ledgerstate.UnlockBlocksValid(inputs, tx) {
		return xerrors.Errorf("spending of referenced consumedOutputs is not authorized")
	}
	return nil
}
