package ledgerstate

import (
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAliasBasic(t *testing.T) {
	bals := NewColoredBalances(map[Color]uint64{ColorIOTA: 10})
	kp := ed25519.GenerateKeyPair()
	addr := NewED25519Address(kp.PublicKey)
	out, err := NewAliasOutputMint(*bals, addr, nil)
	require.NoError(t, err)

	t.Logf("%s", out)
}
