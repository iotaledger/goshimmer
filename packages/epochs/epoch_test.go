package epochs

import (
	"fmt"
	"testing"

	"github.com/iotaledger/hive.go/identity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_EpochMarshaling(t *testing.T) {
	epoch := New(1)
	id, _ := identity.RandomID()
	epoch.AddNode(id)

	epochFromBytes, _, err := EpochFromBytes(epoch.Bytes())
	require.NoError(t, err)

	assert.Equal(t, epoch.Bytes(), epochFromBytes.Bytes())
	assert.Equal(t, epoch.TotalMana(), epochFromBytes.TotalMana())
	assert.Equal(t, epoch.mana[id], epochFromBytes.mana[id])

	fmt.Println(epoch)
}
