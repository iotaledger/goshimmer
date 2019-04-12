package saltmanager

import (
    "github.com/iotaledger/goshimmer/plugins/autopeering/types/salt"
    "time"
)

func CheckSalt(saltToCheck *salt.Salt) error {
    now := time.Now()
    if saltToCheck.ExpirationTime.Before(now.Add(-1 * time.Minute)) {
        return ErrPublicSaltExpired
    }
    if saltToCheck.ExpirationTime.After(now.Add(PUBLIC_SALT_LIFETIME + 1 * time.Minute)) {
        return ErrPublicSaltInvalidLifetime
    }

    return nil
}
