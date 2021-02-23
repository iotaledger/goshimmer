package tangle

import (
	"fmt"
	"time"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/identity"
)

func newTestNonceMessage(nonce uint64) *Message {
	return NewMessage([]MessageID{EmptyMessageID}, []MessageID{}, time.Time{}, ed25519.PublicKey{}, 0, payload.NewGenericDataPayload([]byte("test")), nonce, ed25519.Signature{})
}

func newTestDataMessage(payloadString string) *Message {
	return NewMessage([]MessageID{EmptyMessageID}, []MessageID{}, time.Now(), ed25519.PublicKey{}, 0, payload.NewGenericDataPayload([]byte(payloadString)), 0, ed25519.Signature{})
}

func newTestParentsDataMessage(payloadString string, strongParents, weakParents []MessageID) *Message {
	return NewMessage(strongParents, weakParents, time.Now(), ed25519.PublicKey{}, 0, payload.NewGenericDataPayload([]byte(payloadString)), 0, ed25519.Signature{})
}

func newTestParentsDataWithTimestamp(payloadString string, strongParents, weakParents []MessageID, timestamp time.Time) *Message {
	return NewMessage(strongParents, weakParents, timestamp, ed25519.PublicKey{}, 0, payload.NewGenericDataPayload([]byte(payloadString)), 0, ed25519.Signature{})
}

func newTestParentsPayloadMessage(payload payload.Payload, strongParents, weakParents []MessageID) *Message {
	return NewMessage(strongParents, weakParents, time.Now(), ed25519.PublicKey{}, 0, payload, 0, ed25519.Signature{})
}

func newTestParentsPayloadWithTimestamp(payload payload.Payload, strongParents, weakParents []MessageID, timestamp time.Time) *Message {
	return NewMessage(strongParents, weakParents, timestamp, ed25519.PublicKey{}, 0, payload, 0, ed25519.Signature{})
}

type wallet struct {
	keyPair ed25519.KeyPair
	address *ledgerstate.ED25519Address
}

func (w wallet) privateKey() ed25519.PrivateKey {
	return w.keyPair.PrivateKey
}

func (w wallet) publicKey() ed25519.PublicKey {
	return w.keyPair.PublicKey
}

func createWallets(n int) []wallet {
	wallets := make([]wallet, n)
	for i := 0; i < n; i++ {
		kp := ed25519.GenerateKeyPair()
		wallets[i] = wallet{
			kp,
			ledgerstate.NewED25519Address(kp.PublicKey),
		}
	}
	return wallets
}

func (w wallet) sign(txEssence *ledgerstate.TransactionEssence) *ledgerstate.ED25519Signature {
	return ledgerstate.NewED25519Signature(w.publicKey(), ed25519.Signature(w.privateKey().Sign(txEssence.Bytes())))
}

func (w wallet) unlockBlocks(txEssence *ledgerstate.TransactionEssence) []ledgerstate.UnlockBlock {
	unlockBlock := ledgerstate.NewSignatureUnlockBlock(w.sign(txEssence))
	unlockBlocks := make([]ledgerstate.UnlockBlock, len(txEssence.Inputs()))
	for i := range txEssence.Inputs() {
		unlockBlocks[i] = unlockBlock
	}
	return unlockBlocks
}

// addressFromInput retrieves the Address belonging to an Input by looking it up in the outputs that we have created for
// the tests.
func addressFromInput(input ledgerstate.Input, outputsByID ledgerstate.OutputsByID) ledgerstate.Address {
	typeCastedInput, ok := input.(*ledgerstate.UTXOInput)
	if !ok {
		panic("unexpected Input type")
	}

	switch referencedOutput := outputsByID[typeCastedInput.ReferencedOutputID()]; referencedOutput.Type() {
	case ledgerstate.SigLockedSingleOutputType:
		typeCastedOutput, ok := referencedOutput.(*ledgerstate.SigLockedSingleOutput)
		if !ok {
			panic("failed to type cast SigLockedSingleOutput")
		}

		return typeCastedOutput.Address()
	case ledgerstate.SigLockedColoredOutputType:
		typeCastedOutput, ok := referencedOutput.(*ledgerstate.SigLockedColoredOutput)
		if !ok {
			panic("failed to type cast SigLockedColoredOutput")
		}
		return typeCastedOutput.Address()
	default:
		panic("unexpected Output type")
	}
}

func messageBranchID(tangle *Tangle, messageID MessageID) (branchID ledgerstate.BranchID, err error) {
	if !tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
		branchID = messageMetadata.BranchID()
		// fmt.Println(messageID)
		// fmt.Println(messageMetadata.StructureDetails())
	}) {
		return branchID, fmt.Errorf("missing message metadata")
	}
	return
}

func transactionBranchID(tangle *Tangle, transactionID ledgerstate.TransactionID) (branchID ledgerstate.BranchID, err error) {
	if !tangle.LedgerState.TransactionMetadata(transactionID).Consume(func(metadata *ledgerstate.TransactionMetadata) {
		branchID = metadata.BranchID()
	}) {
		return branchID, fmt.Errorf("missing transaction metadata")
	}
	return
}

func makeTransaction(inputs ledgerstate.Inputs, outputs ledgerstate.Outputs, outputsByID map[ledgerstate.OutputID]ledgerstate.Output, walletsByAddress map[ledgerstate.Address]wallet, genesisWallet ...wallet) *ledgerstate.Transaction {
	txEssence := ledgerstate.NewTransactionEssence(0, time.Now(), identity.ID{}, identity.ID{}, inputs, outputs)
	unlockBlocks := make([]ledgerstate.UnlockBlock, len(txEssence.Inputs()))
	for i, input := range txEssence.Inputs() {
		w := wallet{}
		if genesisWallet != nil {
			w = genesisWallet[0]
		} else {
			w = walletsByAddress[addressFromInput(input, outputsByID)]
		}
		unlockBlocks[i] = ledgerstate.NewSignatureUnlockBlock(w.sign(txEssence))
	}
	return ledgerstate.NewTransaction(txEssence, unlockBlocks)
}

func selectIndex(transaction *ledgerstate.Transaction, w wallet) (index uint16) {
	for i, output := range transaction.Essence().Outputs() {
		if w.address == output.(*ledgerstate.SigLockedSingleOutput).Address() {
			return uint16(i)
		}
	}
	return
}
