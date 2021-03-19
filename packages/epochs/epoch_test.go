package epochs

import (
	"fmt"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/identity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_EpochMarshaling(t *testing.T) {
	epoch := NewEpoch(1)
	id, _ := identity.RandomID()
	epoch.AddNode(id)

	epochFromBytes, _, err := EpochFromBytes(epoch.Bytes())
	require.NoError(t, err)

	assert.Equal(t, epoch.Bytes(), epochFromBytes.Bytes())
	assert.Equal(t, epoch.TotalMana(), epochFromBytes.TotalMana())
	assert.Equal(t, epoch.mana[id], epochFromBytes.mana[id])

	fmt.Println(epoch)
}

func Test_timeToEpochID(t *testing.T) {
	fmt.Println(timeToEpochID(time.Now()))
	fmt.Println(timeToEpochID(time.Now().Add(15 * time.Minute)))
	fmt.Println(timeToEpochID(time.Now().Add(-24 * time.Hour)))
	fmt.Println(timeToEpochID(time.Now().Add(2 * time.Hour)))
}
