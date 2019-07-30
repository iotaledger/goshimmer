package saltmanager

import (
	"time"

	"github.com/dgraph-io/badger"
	"github.com/iotaledger/goshimmer/packages/daemon"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/packages/settings"
	"github.com/iotaledger/goshimmer/plugins/autopeering/types/salt"
)

var (
	PRIVATE_SALT *salt.Salt
	PUBLIC_SALT  *salt.Salt
)

func Configure(plugin *node.Plugin) {
	PRIVATE_SALT = createSalt(PRIVATE_SALT_SETTINGS_KEY, PRIVATE_SALT_LIFETIME, Events.UpdatePrivateSalt.Trigger)
	PUBLIC_SALT = createSalt(PUBLIC_SALT_SETTINGS_KEY, PUBLIC_SALT_LIFETIME, Events.UpdatePublicSalt.Trigger)
}

func generateNewSalt(key []byte, lifetime time.Duration) *salt.Salt {
	newSalt := salt.New(lifetime)

	if err := settings.Set(key, newSalt.Marshal()); err != nil {
		panic(err)
	}

	return newSalt
}

func getSalt(key []byte, lifetime time.Duration) *salt.Salt {
	saltBytes, err := settings.Get(key)
	if err != nil {
		if err == badger.ErrKeyNotFound {
			return generateNewSalt(key, lifetime)
		} else {
			panic(err)
		}
	}

	if resultingSalt, err := salt.Unmarshal(saltBytes); err != nil {
		panic(err)
	} else {
		return resultingSalt
	}
}

func updatePublicSalt(saltToUpdate *salt.Salt, settingsKey []byte, lifeSpan time.Duration, updateCallback func(params ...interface{})) {
	newSalt := salt.New(lifeSpan)

	saltToUpdate.Bytes = newSalt.Bytes
	saltToUpdate.ExpirationTime = newSalt.ExpirationTime

	if err := settings.Set(settingsKey, saltToUpdate.Marshal()); err != nil {
		panic(err)
	}

	updateCallback(saltToUpdate)

	scheduleUpdateForSalt(saltToUpdate, settingsKey, lifeSpan, updateCallback)
}

func scheduleUpdateForSalt(saltToUpdate *salt.Salt, settingsKey []byte, lifeSpan time.Duration, callback func(params ...interface{})) {
	now := time.Now()

	if saltToUpdate.ExpirationTime.Before(now) {
		updatePublicSalt(saltToUpdate, settingsKey, lifeSpan, callback)
	} else {
		daemon.BackgroundWorker("Salt Updater", func() {
			select {
			case <-time.After(saltToUpdate.ExpirationTime.Sub(now)):
				updatePublicSalt(saltToUpdate, settingsKey, lifeSpan, callback)
			case <-daemon.ShutdownSignal:
				return
			}
		})
	}
}

func createSalt(settingsKey []byte, lifeSpan time.Duration, updateCallback func(params ...interface{})) *salt.Salt {
	newSalt := getSalt(settingsKey, lifeSpan)

	scheduleUpdateForSalt(newSalt, settingsKey, lifeSpan, updateCallback)

	return newSalt
}
