package accountability

import (
	"sync"

	"github.com/dgraph-io/badger"
	"github.com/iotaledger/goshimmer/packages/identity"
	"github.com/iotaledger/goshimmer/packages/settings"
)

// Name of the key under which the node identity is stored.
const identityKey = "IDENTITY"

var ownId *identity.Identity
var lazyInit sync.Once

func OwnId() *identity.Identity {
	lazyInit.Do(initOwnId)

	return ownId
}

func initOwnId() {
	ownId = getIdentity()
}

func generateNewIdentity() *identity.Identity {

	newIdentity := identity.GeneratePrivateIdentity()

	key := []byte(identityKey)
	value := newIdentity.Marshal()

	if err := settings.Set(key, value); err != nil {
		panic(err)
	}

	return newIdentity
}

func getIdentity() *identity.Identity {
	key := []byte(identityKey)

	value, err := settings.Get(key)
	if err != nil {
		if err == badger.ErrKeyNotFound {
			return generateNewIdentity()
		} else {
			panic(err)
		}
	}

	result, err := identity.Unmarshal(value)
	if err != nil {
		panic(err)
	}

	return result
}
