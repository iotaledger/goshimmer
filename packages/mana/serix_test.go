package mana

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSerixPersistableBaseMana(t *testing.T) {
	baseMana := newPersistableMana()
	baseMana.NodeID = randomNodeID()

	assert.Equal(t, baseMana.BytesOld(), baseMana.Bytes())
	assert.Equal(t, baseMana.ObjectStorageKeyOld(), baseMana.ObjectStorageKey())
	assert.Equal(t, baseMana.ObjectStorageValueOld(), baseMana.ObjectStorageValue())

	restoredObj, err := new(PersistableBaseMana).FromBytes(baseMana.Bytes())
	assert.NoError(t, err)
	assert.Equal(t, baseMana.Bytes(), restoredObj.Bytes())

	restoredObj2, err := new(PersistableBaseMana).FromObjectStorage(baseMana.ObjectStorageKey(), baseMana.ObjectStorageValue())
	assert.NoError(t, err)
	assert.Equal(t, baseMana.Bytes(), restoredObj2.(*PersistableBaseMana).Bytes())
}
