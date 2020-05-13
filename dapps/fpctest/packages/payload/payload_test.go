package payload

import (
	"crypto/rand"
	"fmt"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/hive.go/identity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ExamplePayload() {
	nonce := make([]byte, NonceSize)
	rand.Read(nonce)
	fpcTestPayload := New(100, nonce)

	localIdentity := identity.GenerateLocalIdentity()
	msg := message.New(
		// trunk in "network tangle" ontology (filled by tipSelector)
		message.EmptyId,

		// branch in "network tangle" ontology (filled by tipSelector)
		message.EmptyId,

		// issuer of the transaction (signs automatically)
		localIdentity,

		// the time when the transaction was created
		time.Now(),

		// the ever increasing sequence number of this transaction
		0,

		// payload
		fpcTestPayload,
	)

	fmt.Println(msg)
}

func TestPayload(t *testing.T) {
	nonce := make([]byte, NonceSize)
	_, err := rand.Read(nonce)
	require.NoError(t, err)
	originalPayload := New(100, nonce)

	clonedPayload1, _, err := FromBytes(originalPayload.Bytes())
	if err != nil {
		panic(err)
	}

	assert.Equal(t, originalPayload.ID(), clonedPayload1.ID())
	assert.Equal(t, originalPayload.Nonce(), clonedPayload1.Nonce())
	assert.Equal(t, originalPayload.Like(), clonedPayload1.Like())
	assert.Equal(t, uint32(100), clonedPayload1.Like())
}
