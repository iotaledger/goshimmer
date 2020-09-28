package ledgerstate

import (
	"fmt"
	"testing"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/stretchr/testify/require"
)

func TestUnlockBlocks_String(t *testing.T) {
	dataToSign := []byte("TEST DATA TO SIGN")

	publicKey, privateKey, err := ed25519.GenerateKey()
	require.NoError(t, err)
	signature := privateKey.Sign(dataToSign)

	unlockBlocks := make(UnlockBlocks, 5)
	unlockBlocks[0] = NewSignatureUnlockBlock(NewED25519Signature(publicKey, signature))
	unlockBlocks[1] = NewReferenceUnlockBlock(1)
	unlockBlocks[2] = NewReferenceUnlockBlock(2)

	fmt.Println(unlockBlocks)
}
