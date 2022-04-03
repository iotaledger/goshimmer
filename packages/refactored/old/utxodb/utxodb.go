package utxodb

import (
	"golang.org/x/xerrors"

	"github.com/iotaledger/goshimmer/packages/refactored/ledger/vms/devnetvm"

	"github.com/iotaledger/goshimmer/packages/refactored/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/refactored/old"
)

// Supply returns supply of the instance.
func (u *UtxoDB) Supply() uint64 {
	return u.supply
}

// IsConfirmed checks if the transaction is in the UTXODB ledger.
func (u *UtxoDB) IsConfirmed(txid *utxo.TransactionID) bool {
	u.mutex.Lock()
	defer u.mutex.Unlock()
	_, ok := u.transactions[*txid]
	return ok
}

// GetOutput finds an output by ID (either spent or unspent).
func (u *UtxoDB) GetOutput(outID utxo.OutputID, f func(devnetvm.OutputEssence)) bool {
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
func (u *UtxoDB) GetOutputMetadata(outID utxo.OutputID, f func(*old.OutputMetadata)) bool {
	var out devnetvm.OutputEssence
	u.GetOutput(outID, func(o devnetvm.OutputEssence) {
		out = o
	})
	if out == nil {
		return false
	}
	meta := old.NewOutputMetadata(outID)
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
func (u *UtxoDB) AddTransaction(tx *devnetvm.Transaction) error {
	u.mutex.Lock()
	defer u.mutex.Unlock()

	// serialize/deserialize for proper semantic check
	tx, err := new(devnetvm.Transaction).FromBytes(tx.Bytes())
	if err != nil {
		return err
	}
	if err = u.CheckNewTransaction(tx, false); err != nil {
		return err
	}
	// delete consumed (referenced) outputs from the ledger
	for _, inp := range tx.Essence().Inputs() {
		utxoInp := inp.(*devnetvm.UTXOInput)

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
		u.utxo[out.ID()] = out.UpdateMintingColor()
	}
	u.transactions[tx.ID()] = tx
	u.checkLedgerBalance()
	return nil
}

// GetTransaction retrieves value transaction by its hash (ID).
func (u *UtxoDB) GetTransaction(id utxo.TransactionID) (*devnetvm.Transaction, bool) {
	u.mutex.RLock()
	defer u.mutex.RUnlock()

	return u.getTransaction(id)
}

// MustGetTransaction same as GetTransaction only panics if transaction is not in UTXODB.
func (u *UtxoDB) MustGetTransaction(id utxo.TransactionID) *devnetvm.Transaction {
	u.mutex.RLock()
	defer u.mutex.RUnlock()
	return u.mustGetTransaction(id)
}

// GetAddressOutputs returns unspent outputs contained in the address.
func (u *UtxoDB) GetAddressOutputs(addr devnetvm.Address) []devnetvm.OutputEssence {
	u.mutex.RLock()
	defer u.mutex.RUnlock()

	return u.getAddressOutputs(addr)
}

// GetAddressBalances return all colored balances of the address.
func (u *UtxoDB) GetAddressBalances(addr devnetvm.Address) map[devnetvm.Color]uint64 {
	ret := make(map[devnetvm.Color]uint64)
	outputs := u.GetAddressOutputs(addr)
	for _, out := range outputs {
		out.Balances().ForEach(func(col devnetvm.Color, bal uint64) bool {
			s := ret[col]
			ret[col] = s + bal
			return true
		})
	}
	return ret
}

// Balance returns balances of specific color.
func (u *UtxoDB) Balance(addr devnetvm.Address, color devnetvm.Color) uint64 {
	bals := u.GetAddressBalances(addr)
	ret := bals[color]
	return ret
}

// BalanceIOTA number of iotas in the address.
func (u *UtxoDB) BalanceIOTA(addr devnetvm.Address) uint64 {
	return u.Balance(addr, devnetvm.ColorIOTA)
}

// CollectUnspentOutputsFromInputs returns unspent outputs by inputs of the transaction.
func (u *UtxoDB) CollectUnspentOutputsFromInputs(essence *devnetvm.TransactionEssence) ([]devnetvm.OutputEssence, error) {
	u.mutex.RLock()
	defer u.mutex.RUnlock()

	return u.collectUnspentOutputsFromInputs(essence)
}

// CheckNewTransaction checks consistency of the transaction the same way as ledgerstate.
func (u *UtxoDB) CheckNewTransaction(tx *devnetvm.Transaction, lock ...bool) error {
	if len(lock) > 0 && lock[0] {
		u.mutex.RLock()
		defer u.mutex.RUnlock()
	}
	inputs, err := u.collectUnspentOutputsFromInputs(tx.Essence())
	if err != nil {
		return err
	}
	if !devnetvm.TransactionBalancesValid(inputs, tx.Essence().Outputs()) {
		return xerrors.Errorf("sum of consumed and spent balances is not 0")
	}
	if ok, err := devnetvm.UnlockBlocksValidWithError(inputs, tx); !ok || err != nil {
		return xerrors.Errorf("CheckNewTransaction: input unlocking failed: %v", err)
	}
	return nil
}

// GetAliasOutputs collects all outputs of type ledgerstate.AliasOutput for the transaction.
func (u *UtxoDB) GetAliasOutputs(addr devnetvm.Address) []*devnetvm.AliasOutput {
	outs := u.GetAddressOutputs(addr)
	ret := make([]*devnetvm.AliasOutput, 0)
	for _, out := range outs {
		if o, ok := out.(*devnetvm.AliasOutput); ok {
			ret = append(ret, o)
		}
	}
	return ret
}

// findUnspentOutputByID returns unspent output with existence flag.
func (u *UtxoDB) findUnspentOutputByID(id utxo.OutputID) (devnetvm.OutputEssence, bool) {
	if out, ok := u.utxo[id]; ok {
		return out, true
	}
	return nil, false
}

func (u *UtxoDB) getTransaction(id utxo.TransactionID) (*devnetvm.Transaction, bool) {
	tx, ok := u.transactions[id]
	return tx, ok
}

func (u *UtxoDB) mustGetTransaction(id utxo.TransactionID) *devnetvm.Transaction {
	tx, ok := u.transactions[id]
	if !ok {
		panic(xerrors.Errorf("utxodb.mustGetTransaction: tx id doesn't exist: %s", id.String()))
	}
	return tx
}

func (u *UtxoDB) getAddressOutputs(addr devnetvm.Address) []devnetvm.OutputEssence {
	addrArr := addr.Array()
	ret := make([]devnetvm.OutputEssence, 0)
	for _, out := range u.utxo {
		if out.Address().Array() == addrArr {
			ret = append(ret, out)
		}
	}
	return ret
}

func (u *UtxoDB) getOutputTotal(outid utxo.OutputID) (uint64, error) {
	out, ok := u.utxo[outid]
	if !ok {
		return 0, xerrors.Errorf("getOutputTotal: no such output: %s", outid.String())
	}
	ret := uint64(0)
	out.Balances().ForEach(func(_ devnetvm.Color, bal uint64) bool {
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

func (u *UtxoDB) collectUnspentOutputsFromInputs(essence *devnetvm.TransactionEssence) ([]devnetvm.OutputEssence, error) {
	ret := make([]devnetvm.OutputEssence, len(essence.Inputs()))
	for i, inp := range essence.Inputs() {
		if inp.Type() != devnetvm.UTXOInputType {
			return nil, xerrors.New("CollectUnspentOutputsFromInputs: wrong input type")
		}
		utxoInp := inp.(*devnetvm.UTXOInput)
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
