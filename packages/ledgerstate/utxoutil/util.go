package utxoutil

import (
	"bytes"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/minio/blake2b-simd"
	"golang.org/x/xerrors"
)

func EqualAddresses(a1, a2 ledgerstate.Address) bool {
	if a1 == a2 {
		return true
	}
	if a1 == nil || a2 == nil {
		return false
	}
	if a1.Type() != a2.Type() {
		return false
	}
	if bytes.Compare(a1.Digest(), a2.Digest()) != 0 {
		return false
	}
	return true
}

type signatureUnlockBlockWithIndex struct {
	unlockBlock   *ledgerstate.SignatureUnlockBlock
	indexUnlocked int
}

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

func unlockInputsWithSignatureBlocks(inputs []ledgerstate.Output, sigUnlockBlocks map[[33]byte]*signatureUnlockBlockWithIndex) ([]ledgerstate.UnlockBlock, error) {
	// unlock ChainOutputs
	ret := make([]ledgerstate.UnlockBlock, len(inputs))
	for index, out := range inputs {
		if ret[index] != nil {
			continue
		}
		switch ot := out.(type) {
		case *ledgerstate.ChainOutput:
			sig, ok := sigUnlockBlocks[ot.GetStateAddress().Array()]
			if !ok {
				return nil, xerrors.Errorf("chain input %d can't be unlocked for state update")
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
				if !EqualAddresses(ot.GetAliasAddress(), eot.Address()) {
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
				return nil, xerrors.Errorf("sig locked input %d can't be unlocked")
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
				return nil, xerrors.Errorf("sig locked input %d can't be unlocked")
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
			return nil, xerrors.Errorf("unsupported output type at #d", index)
		}
	}
	for _, b := range ret {
		if b == nil {
			return nil, xerrors.New("failed to unlock some inputs")
		}
	}
	return ret, nil
}

// CollectChainedOutputs scans all outputs and collects ledgerstate.ChainOutput into a map by the Address.Array
// Returns an error if finds duplicate
func CollectChainedOutputs(essence *ledgerstate.TransactionEssence) (map[[33]byte]*ledgerstate.ChainOutput, error) {
	ret := make(map[[33]byte]*ledgerstate.ChainOutput)
	for _, o := range essence.Outputs() {
		out, ok := o.(*ledgerstate.ChainOutput)
		if !ok {
			continue
		}
		if _, ok := ret[out.GetAliasAddress().Array()]; ok {
			return nil, xerrors.New("duplicate chain output")
		}
		ret[out.GetAliasAddress().Array()] = out
	}
	return ret, nil
}

// GetSingleChainedOutput expects the exactly one chained output in the transaction and returns it
// returns:
// - nil and no error if found none
// - error if there's more than 1
func GetSingleChainedOutput(essence *ledgerstate.TransactionEssence) (*ledgerstate.ChainOutput, error) {
	ch, err := CollectChainedOutputs(essence)
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
// if it has a single alias input (i.e. output is not an origin) it returns a alias address of the chain
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
	chained, err := GetSingleChainedOutput(tx.Essence())
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
