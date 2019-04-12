package saltmanager

import (
    "github.com/dgraph-io/badger"
    "github.com/iotaledger/goshimmer/packages/daemon"
    "github.com/iotaledger/goshimmer/packages/settings"
    "github.com/iotaledger/goshimmer/plugins/autopeering/types/salt"
    "time"
)

var (
    PRIVATE_SALT = createSalt(PRIVATE_SALT_SETTINGS_KEY, PRIVATE_SALT_LIFETIME, Events.UpdatePrivateSalt.Trigger)
    PUBLIC_SALT  = createSalt(PUBLIC_SALT_SETTINGS_KEY, PUBLIC_SALT_LIFETIME, Events.UpdatePublicSalt.Trigger)
)

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

func updatePublicSalt(saltToUpdate *salt.Salt, settingsKey []byte, lifeSpan time.Duration, updateCallback func(salt *salt.Salt)) {
    newSalt := salt.New(lifeSpan)

    saltToUpdate.Bytes = newSalt.Bytes
    saltToUpdate.ExpirationTime = newSalt.ExpirationTime

    if err := settings.Set(settingsKey, saltToUpdate.Marshal()); err != nil {
        panic(err)
    }

    updateCallback(saltToUpdate)

    scheduleUpdateForSalt(saltToUpdate, settingsKey, lifeSpan, updateCallback)
}

func scheduleUpdateForSalt(saltToUpdate *salt.Salt, settingsKey []byte, lifeSpan time.Duration, callback SaltConsumer) {
    now := time.Now()

    if saltToUpdate.ExpirationTime.Before(now) {
        updatePublicSalt(saltToUpdate, settingsKey, lifeSpan, callback)
    } else {
        go func() {
            select {
            case <-daemon.ShutdownSignal:
                return
            case <-time.After(saltToUpdate.ExpirationTime.Sub(now)):
                updatePublicSalt(saltToUpdate, settingsKey, lifeSpan, callback)
            }
        }()
    }
}

func createSalt(settingsKey []byte, lifeSpan time.Duration, updateCallback SaltConsumer) *salt.Salt {
    newSalt := getSalt(settingsKey, lifeSpan)

    scheduleUpdateForSalt(newSalt, settingsKey, lifeSpan, updateCallback)

    return newSalt
}
