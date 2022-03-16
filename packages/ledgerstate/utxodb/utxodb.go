package utxodb

import (
	"golang.org/x/xerrors"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

// Supply returns supply of the instance.
func (u *UtxoDB) Supply() uint64 {
	return u.supply
}

// IsConfirmed checks if the transaction is in the UTXODB ledger.
func (u *UtxoDB) IsConfirmed(txid *ledgerstate.TransactionID) bool {
	u.mutex.Lock()
	defer u.mutex.Unlock()
	_, ok := u.transactions[*txid]
	return ok
}

// GetOutput finds an output by ID (either spent or unspent).
func (u *UtxoDB) GetOutput(outID ledgerstate.OutputID, f func(ledgerstate.Output)) bool {
	out, ok := u.utxo[outID]
	if ok {
		f(out)
		return true
	}
	out, ok = u.consumedOutputs[outID]
	if ok {
		f(out)
		return true
	}
	return false
}

// GetOutputMetadata finds an output by ID and returns its (mocked) metadata.
func (u *UtxoDB) GetOutputMetadata(outID ledgerstate.OutputID, f func(*ledgerstate.OutputMetadata)) bool {
	var out ledgerstate.Output
	u.GetOutput(outID, func(o ledgerstate.Output) {
		out = o
	})
	if out == nil {
		return false
	}
	meta := ledgerstate.NewOutputMetadata(outID)
	txID, consumed := u.consumedBy[outID]
	if consumed {
		meta.RegisterConsumer(txID)
	}
	meta.SetSolid(true)
	f(meta)
	return true
}

// AddTransaction adds transaction to UTXODB or return an error.
// The function ensures consistency of the UTXODB ledger.
func (u *UtxoDB) AddTransaction(tx *ledgerstate.Transaction) error {
	u.mutex.Lock()
	defer u.mutex.Unlock()

	// serialize/deserialize for proper semantic check
	tx, err := new(ledgerstate.Transaction).FromBytes(tx.Bytes())
	if err != nil {
		return err
	}
	if err = u.CheckNewTransaction(tx, false); err != nil {
		return err
	}
	// delete consumed (referenced) outputs from the ledger
	for _, inp := range tx.Essence().Inputs() {
		utxoInp := inp.(*ledgerstate.UTXOInput)

		consumed, ok := u.findUnspentOutputByID(utxoInp.ReferencedOutputID())
		if !ok {
			return xerrors.Errorf("deleting UTXO: corresponding output does not exists: %s", utxoInp.ReferencedOutputID().String())
		}
		delete(u.utxo, utxoInp.ReferencedOutputID())
		u.consumedOutputs[utxoInp.ReferencedOutputID()] = consumed
		u.consumedBy[utxoInp.ReferencedOutputID()] = tx.ID()
	}
	// add outputs to the ledger
	for _, out := range tx.Essence().Outputs() {
		if out.ID().TransactionID() != tx.ID() {
			panic("utxodb.AddTransaction: incorrect output ID")
		}
		u.utxo[out.ID()] = out.UpdateMintingColor()
	}
	u.transactions[tx.ID()] = tx
	u.checkLedgerBalance()
	return nil
}

// GetTransaction retrieves value transaction by its hash (ID).
func (u *UtxoDB) GetTransaction(id ledgerstate.TransactionID) (*ledgerstate.Transaction, bool) {
	u.mutex.RLock()
	defer u.mutex.RUnlock()

	return u.getTransaction(id)
}

// MustGetTransaction same as GetTransaction only panics if transaction is not in UTXODB.
func (u *UtxoDB) MustGetTransaction(id ledgerstate.TransactionID) *ledgerstate.Transaction {
	u.mutex.RLock()
	defer u.mutex.RUnlock()
	return u.mustGetTransaction(id)
}

// GetAddressOutputs returns unspent outputs contained in the address.
func (u *UtxoDB) GetAddressOutputs(addr ledgerstate.Address) []ledgerstate.Output {
	u.mutex.RLock()
	defer u.mutex.RUnlock()

	return u.getAddressOutputs(addr)
}

// GetAddressBalances return all colored balances of the address.
func (u *UtxoDB) GetAddressBalances(addr ledgerstate.Address) map[ledgerstate.Color]uint64 {
	ret := make(map[ledgerstate.Color]uint64)
	outputs := u.GetAddressOutputs(addr)
	for _, out := range outputs {
		out.Balances().ForEach(func(col ledgerstate.Color, bal uint64) bool {
			s := ret[col]
			ret[col] = s + bal
			return true
		})
	}
	return ret
}

// Balance returns balances of specific color.
func (u *UtxoDB) Balance(addr ledgerstate.Address, color ledgerstate.Color) uint64 {
	bals := u.GetAddressBalances(addr)
	ret := bals[color]
	return ret
}

// BalanceIOTA number of iotas in the address.
func (u *UtxoDB) BalanceIOTA(addr ledgerstate.Address) uint64 {
	return u.Balance(addr, ledgerstate.ColorIOTA)
}

// CollectUnspentOutputsFromInputs returns unspent outputs by inputs of the transaction.
func (u *UtxoDB) CollectUnspentOutputsFromInputs(essence *ledgerstate.TransactionEssence) ([]ledgerstate.Output, error) {
	u.mutex.RLock()
	defer u.mutex.RUnlock()

	return u.collectUnspentOutputsFromInputs(essence)
}

// CheckNewTransaction checks consistency of the transaction the same way as ledgerstate.
func (u *UtxoDB) CheckNewTransaction(tx *ledgerstate.Transaction, lock ...bool) error {
	if len(lock) > 0 && lock[0] {
		u.mutex.RLock()
		defer u.mutex.RUnlock()
	}
	inputs, err := u.collectUnspentOutputsFromInputs(tx.Essence())
	if err != nil {
		return err
	}
	if !ledgerstate.TransactionBalancesValid(inputs, tx.Essence().Outputs()) {
		return xerrors.Errorf("sum of consumed and spent balances is not 0")
	}
	if ok, err := ledgerstate.UnlockBlocksValidWithError(inputs, tx); !ok || err != nil {
		return xerrors.Errorf("CheckNewTransaction: input unlocking failed: %v", err)
	}
	return nil
}

// GetAliasOutputs collects all outputs of type ledgerstate.AliasOutput for the transaction.
func (u *UtxoDB) GetAliasOutputs(addr ledgerstate.Address) []*ledgerstate.AliasOutput {
	outs := u.GetAddressOutputs(addr)
	ret := make([]*ledgerstate.AliasOutput, 0)
	for _, out := range outs {
		if o, ok := out.(*ledgerstate.AliasOutput); ok {
			ret = append(ret, o)
		}
	}
	return ret
}

// findUnspentOutputByID returns unspent output with existence flag.
func (u *UtxoDB) findUnspentOutputByID(id ledgerstate.OutputID) (ledgerstate.Output, bool) {
	if out, ok := u.utxo[id]; ok {
		return out, true
	}
	return nil, false
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
		return 0, xerrors.Errorf("getOutputTotal: no such output: %s", outid.String())
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
	if total != defaultSupply {
		panic("utxodb: wrong ledger balance")
	}
}

func (u *UtxoDB) collectUnspentOutputsFromInputs(essence *ledgerstate.TransactionEssence) ([]ledgerstate.Output, error) {
	ret := make([]ledgerstate.Output, len(essence.Inputs()))
	for i, inp := range essence.Inputs() {
		if inp.Type() != ledgerstate.UTXOInputType {
			return nil, xerrors.New("CollectUnspentOutputsFromInputs: wrong input type")
		}
		utxoInp := inp.(*ledgerstate.UTXOInput)
		var ok bool
		oid := utxoInp.ReferencedOutputID()
		if ret[i], ok = u.findUnspentOutputByID(oid); !ok {
			return nil, xerrors.Errorf("CollectUnspentOutputsFromInputs: unspent output does not exist: %s", oid.String())
		}
		otx, ok := u.getTransaction(oid.TransactionID())
		if !ok {
			return nil, xerrors.Errorf("CollectUnspentOutputsFromInputs: input transaction not found: %s", oid.TransactionID())
		}
		if essence.Timestamp().Before(otx.Essence().Timestamp()) {
			return nil, xerrors.Errorf("CollectUnspentOutputsFromInputs: transaction timestamp is before input timestamp: %s", oid.TransactionID())
		}
	}
	return ret, nil
}
