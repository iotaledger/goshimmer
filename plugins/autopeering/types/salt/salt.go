package salt

import (
	"crypto/rand"
	"time"

	"github.com/pkg/errors"
)

type Salt struct {
	Bytes          []byte
	ExpirationTime time.Time
}

func New(lifetime time.Duration) *Salt {
	salt := &Salt{
		Bytes:          make([]byte, SALT_BYTES_SIZE),
		ExpirationTime: time.Now().Add(lifetime),
	}

	if _, err := rand.Read(salt.Bytes); err != nil {
		panic(err)
	}

	return salt
}

func Unmarshal(marshaledSalt []byte) (*Salt, error) {
	if len(marshaledSalt) < SALT_MARSHALED_SIZE {
		return nil, errors.New("marshaled salt bytes not long enough")
	}

	salt := &Salt{
		Bytes: make([]byte, SALT_BYTES_SIZE),
	}
	copy(salt.Bytes, marshaledSalt[SALT_BYTES_START:SALT_BYTES_END])

	if err := salt.ExpirationTime.UnmarshalBinary(marshaledSalt[SALT_TIME_START:SALT_TIME_END]); err != nil {
		return nil, err
	}

	return salt, nil
}

func (this *Salt) Marshal() []byte {
	result := make([]byte, SALT_BYTES_SIZE+SALT_TIME_SIZE)

	copy(result[SALT_BYTES_START:SALT_BYTES_END], this.Bytes)

	if bytes, err := this.ExpirationTime.MarshalBinary(); err != nil {
		panic(err)
	} else {
		copy(result[SALT_TIME_START:SALT_TIME_END], bytes)
	}

	return result
}
