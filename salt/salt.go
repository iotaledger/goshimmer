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

func NewSalt(lifetime time.Duration) (salt *Salt, err error) {
	salt = &Salt{
		Bytes:          make([]byte, SaltByteSize),
		ExpirationTime: time.Now().Add(lifetime),
	}

	if _, err = rand.Read(salt.Bytes); err != nil {
		return nil, err
	}

	return salt, err
}

func (s *Salt) Expired() bool {
	return time.Now().After(s.ExpirationTime)
}
