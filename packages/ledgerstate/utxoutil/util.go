package utxoutil

import (
	"bytes"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/hive.go/crypto/ed25519"
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

func UnlockInputsWithED25519KeyPairs(inputs []ledgerstate.Output, essence *ledgerstate.TransactionEssence, keyPairs []*ed25519.KeyPair) ([]ledgerstate.UnlockBlock, error) {
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
				ret[index] = ledgerstate.NewAliasReferenceUnlockBlock(uint16(sig.indexUnlocked))
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
				ret[i] = ledgerstate.NewAliasReferenceUnlockBlock(uint16(index))
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

func GetSingleChainedOutput(essence *ledgerstate.TransactionEssence) (*ledgerstate.ChainOutput, error) {
	ch, err := CollectChainedOutputs(essence)
	if err != nil {
		return nil, err
	}
	if len(ch) == 0 {
		return nil, xerrors.New("chained output not found")
	}
	if len(ch) > 1 {
		return nil, xerrors.New("more than one chained output was found")
	}
	for _, out := range ch {
		return out, nil
	}
	panic("shouldn't be here")
}
