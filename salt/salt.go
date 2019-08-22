package salt

import (
	"crypto/rand"
	"time"
)

const SaltByteSize = 20

type Salt struct {
	Bytes          []byte
	ExpirationTime time.Time
}

func New(lifetime time.Duration) (salt *Salt, err error) {
	salt = &Salt{
		Bytes:          make([]byte, SaltByteSize),
		ExpirationTime: time.Now().Add(lifetime),
	}

	if _, err = rand.Read(salt.Bytes); err != nil {
		return nil, err
	}

	return salt, err
}
