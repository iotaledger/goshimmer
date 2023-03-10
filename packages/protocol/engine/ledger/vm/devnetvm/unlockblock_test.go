package devnetvm

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/hive.go/crypto/ed25519"
)

func TestUnlockBlockFromBytes(t *testing.T) {
	keyPair := ed25519.GenerateKeyPair()

	// test a valid set of UnlockBlocks
	{
		unlockBlocks := UnlockBlocks{
			NewSignatureUnlockBlock(NewED25519Signature(keyPair.PublicKey, keyPair.PrivateKey.Sign([]byte("testdata")))),
			NewReferenceUnlockBlock(0),
		}
		marshaledUnlockBlocks := unlockBlocks.Bytes()
		parsedUnlockBlocks, consumedBytes, err := UnlockBlocksFromBytes(marshaledUnlockBlocks)

		require.NoError(t, err)
		require.Equal(t, len(marshaledUnlockBlocks), consumedBytes)
		require.Equal(t, unlockBlocks, parsedUnlockBlocks)
	}

	// test an invalid set of UnlockBlocks
	// TODO: this should be enabled again once duplicate validation is enabled
	// {
	// 	unlockBlocks := UnlockBlocks{
	// 		NewSignatureUnlockBlock(NewED25519Signature(keyPair.PublicKey, keyPair.PrivateKey.Sign([]byte("testdata")))),
	// 		NewSignatureUnlockBlock(NewED25519Signature(keyPair.PublicKey, keyPair.PrivateKey.Sign([]byte("testdata")))),
	// 	}
	// 	_, _, err := UnlockBlocksFromBytes(unlockBlocks.Bytes())
	// 	require.Error(t, err)
	// }
}
