package utxoutil

import (
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"golang.org/x/crypto/blake2b"
	"golang.org/x/xerrors"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

// signatureUnlockBlockWithIndex internal structure used to track signature and indices where it was used in the transaction.
type signatureUnlockBlockWithIndex struct {
	unlockBlock   *ledgerstate.SignatureUnlockBlock
	indexUnlocked int
}

// UnlockInputsWithED25519KeyPairs signs the transaction essence with provided ED25519 pair. Then it unlocks
// inputs provided as a list of outputs using those signatures and returns a list of unlock blocks in the same
// order as inputs.
// It only uses no more signatures as it is needed to unlock all inputs. Other unlock blocks are references
// The function unlocks the following output types:
// - ledgerstate.AliasOutput
// - ledgerstate.ExtendedLockedOutput
// - ledgerstate.SigLockedSingleOutput
// - ledgerstate.SigLockedColoredOutput
// It unlocks inputs by using the following unlock block types:
// - ledgerstate.SignatureUnlockBlock
// - ledgerstate.ReferenceUnlockBlock
// - ledgerstate.AliasUnlockBlock.
func UnlockInputsWithED25519KeyPairs(inputs []ledgerstate.Output, essence *ledgerstate.TransactionEssence, keyPairs ...*ed25519.KeyPair) ([]ledgerstate.UnlockBlock, error) {
	sigs := make(map[[33]byte]*signatureUnlockBlockWithIndex)
	for _, keyPair := range keyPairs {
		addr := ledgerstate.NewED25519Address(keyPair.PublicKey)
		data := essence.Bytes()
		signature := ledgerstate.NewED25519Signature(keyPair.PublicKey, keyPair.PrivateKey.Sign(data))
		if !signature.AddressSignatureValid(addr, data) {
			panic("SigUnlockBlockED25519: internal error, unlockBlock invalid")
		}
		sigs[addr.Array()] = &signatureUnlockBlockWithIndex{
			unlockBlock:   ledgerstate.NewSignatureUnlockBlock(signature),
			indexUnlocked: -1,
		}
	}
	return unlockInputsWithSignatureBlocks(inputs, sigs)
}

// unlockInputsWithSignatureBlocks does the optimized unlocking.
func unlockInputsWithSignatureBlocks(inputs []ledgerstate.Output, sigUnlockBlocks map[[33]byte]*signatureUnlockBlockWithIndex) ([]ledgerstate.UnlockBlock, error) {
	// unlock ChainOutputs
	ret := make([]ledgerstate.UnlockBlock, len(inputs))
	for index, out := range inputs {
		if ret[index] != nil {
			continue
		}
		switch ot := out.(type) {
		case *ledgerstate.AliasOutput:
			sig, ok := sigUnlockBlocks[ot.GetStateAddress().Array()]
			if !ok {
				return nil, xerrors.New("chain input can't be unlocked for state update")
			}
			if sig.indexUnlocked >= 0 {
				// signature already included
				ret[index] = ledgerstate.NewAliasUnlockBlock(uint16(sig.indexUnlocked))
			} else {
				// signature is included here
				ret[index] = sig.unlockBlock
				sig.indexUnlocked = index
			}
			// assign unlock blocks for all alias locked inputs
			for i, o := range inputs {
				eot, ok := o.(*ledgerstate.ExtendedLockedOutput)
				if !ok {
					continue
				}
				if !ot.GetAliasAddress().Equals(eot.Address()) {
					continue
				}
				ret[i] = ledgerstate.NewAliasUnlockBlock(uint16(index))
			}

		case *ledgerstate.ExtendedLockedOutput:
			sig, ok := sigUnlockBlocks[ot.Address().Array()]
			if !ok {
				// no corresponding signature, it probably is an alias
				continue
			}
			if sig.indexUnlocked >= 0 {
				// signature already included
				ret[index] = ledgerstate.NewReferenceUnlockBlock(uint16(sig.indexUnlocked))
			} else {
				// signature is included here
				ret[index] = sig.unlockBlock
				sig.indexUnlocked = index
			}

		case *ledgerstate.SigLockedSingleOutput:
			sig, ok := sigUnlockBlocks[ot.Address().Array()]
			if !ok {
				return nil, xerrors.New("sig locked input can't be unlocked")
			}
			if sig.indexUnlocked >= 0 {
				// signature already included
				ret[index] = ledgerstate.NewReferenceUnlockBlock(uint16(sig.indexUnlocked))
			} else {
				// signature is included here
				ret[index] = sig.unlockBlock
				sig.indexUnlocked = index
			}
		case *ledgerstate.SigLockedColoredOutput:
			sig, ok := sigUnlockBlocks[ot.Address().Array()]
			if !ok {
				return nil, xerrors.New("sig locked input can't be unlocked")
			}
			if sig.indexUnlocked >= 0 {
				// signature already included
				ret[index] = ledgerstate.NewReferenceUnlockBlock(uint16(sig.indexUnlocked))
			} else {
				// signature is included here
				ret[index] = sig.unlockBlock
				sig.indexUnlocked = index
			}
		default:
			return nil, xerrors.Errorf("unsupported output type at index %d", index)
		}
	}
	for _, b := range ret {
		if b == nil {
			return nil, xerrors.New("failed to unlock some inputs")
		}
	}
	return ret, nil
}

// collectAliasOutputs scans all outputs and collects ledgerstate.AliasOutput into a map by the Address.Array
// Returns an error if finds duplicate.
func collectAliasOutputs(tx *ledgerstate.Transaction) (map[[33]byte]*ledgerstate.AliasOutput, error) {
	ret := make(map[[33]byte]*ledgerstate.AliasOutput)
	for i, o := range tx.Essence().Outputs() {
		out, ok := o.(*ledgerstate.AliasOutput)
		if !ok {
			continue
		}
		if _, ok := ret[out.GetAliasAddress().Array()]; ok {
			return nil, xerrors.New("duplicate chain output")
		}
		o = o.SetID(ledgerstate.NewOutputID(tx.ID(), uint16(i))).UpdateMintingColor()
		ret[out.GetAliasAddress().Array()] = o.(*ledgerstate.AliasOutput)
	}
	return ret, nil
}

// GetSingleChainedAliasOutput expects exactly one chained output in the transaction and returns it
// returns:
// - nil and no error if found none
// - error if there's more than 1.
func GetSingleChainedAliasOutput(tx *ledgerstate.Transaction) (*ledgerstate.AliasOutput, error) {
	ch, err := collectAliasOutputs(tx)
	if err != nil {
		return nil, err
	}
	if len(ch) == 0 {
		return nil, nil
	}
	if len(ch) > 1 {
		return nil, xerrors.New("more than one chained output was found")
	}
	for _, out := range ch {
		return out, nil
	}
	panic("shouldn't be here")
}

// GetSingleSender analyzes transaction and signatures and retrieves single address which is consistent
// to be a 'sender':
// if it do not have alias input, the address corresponding to the only signature is returned
// if it has a single alias input (i.e. output is not an origin) it returns a alias address of the chain.
func GetSingleSender(tx *ledgerstate.Transaction) (ledgerstate.Address, error) {
	// only accepting one signature in the transaction
	var sigBlock *ledgerstate.SignatureUnlockBlock
	for _, blk := range tx.UnlockBlocks() {
		t, ok := blk.(*ledgerstate.SignatureUnlockBlock)
		if !ok {
			continue
		}
		if sigBlock != nil {
			return nil, xerrors.New("GetSingleSender: exactly one signature block expected")
		}
		sigBlock = t
	}
	if sigBlock == nil {
		panic("GetSingleSender: exactly one signature block expected")
	}
	addr, err := ledgerstate.AddressFromSignature(sigBlock.Signature())
	if err != nil {
		return nil, err
	}
	chained, err := GetSingleChainedAliasOutput(tx)
	if err != nil {
		return nil, err
	}
	if chained == nil || chained.IsOrigin() {
		// no chained output means the address is the sender
		// if chained output is origin (i.e. has no corresponding input)
		// the address is the sender
		return addr, nil
	}
	return chained.GetAliasAddress(), nil
}

// GetMintedAmounts analyzes outputs and extracts information of new colors
// which were minted and respective amounts of tokens.
func GetMintedAmounts(tx *ledgerstate.Transaction) map[ledgerstate.Color]uint64 {
	ret := make(map[ledgerstate.Color]uint64)
	for _, out := range tx.Essence().Outputs() {
		out.Balances().ForEach(func(col ledgerstate.Color, bal uint64) bool {
			if col == ledgerstate.ColorMint {
				ret[ledgerstate.Color(blake2b.Sum256(out.ID().Bytes()))] = bal
				return false
			}
			return true
		})
	}
	return ret
}
