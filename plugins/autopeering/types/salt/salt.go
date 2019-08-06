package salt

import (
	"crypto/rand"
	"sync"
	"time"

	"github.com/pkg/errors"
)

type Salt struct {
	bytes               []byte
	bytesMutex          sync.RWMutex
	expirationTime      time.Time
	expirationTimeMutex sync.RWMutex
}

func (salt *Salt) GetBytes() (result []byte) {
	salt.bytesMutex.RLock()
	result = make([]byte, len(salt.bytes))
	copy(result[:], salt.bytes[:])
	salt.bytesMutex.RUnlock()

	return
}

func (salt *Salt) SetBytes(b []byte) {
	salt.bytesMutex.Lock()
	salt.bytes = make([]byte, len(b))
	copy(salt.bytes[:], b[:])
	salt.bytesMutex.Unlock()
}

func (salt *Salt) GetExpirationTime() (result time.Time) {
	salt.expirationTimeMutex.RLock()
	result = salt.expirationTime
	salt.expirationTimeMutex.RUnlock()

	return
}

func (salt *Salt) SetExpirationTime(t time.Time) {
	salt.expirationTimeMutex.Lock()
	salt.expirationTime = t
	salt.expirationTimeMutex.Unlock()
}

func New(lifetime time.Duration) *Salt {
	salt := &Salt{
		bytes:          make([]byte, SALT_BYTES_SIZE),
		expirationTime: time.Now().Add(lifetime),
	}

	if _, err := rand.Read(salt.bytes); err != nil {
		panic(err)
	}

	return salt
}

func Unmarshal(marshaledSalt []byte) (*Salt, error) {
	if len(marshaledSalt) < SALT_MARSHALED_SIZE {
		return nil, errors.New("marshaled salt bytes not long enough")
	}

	salt := &Salt{
		bytes: make([]byte, SALT_BYTES_SIZE),
	}
	salt.SetBytes(marshaledSalt[SALT_BYTES_START:SALT_BYTES_END])

	var expTime time.Time
	if err := expTime.UnmarshalBinary(marshaledSalt[SALT_TIME_START:SALT_TIME_END]); err != nil {
		return nil, err
	}
	salt.SetExpirationTime(expTime)

	return salt, nil
}

func (this *Salt) Marshal() []byte {
	result := make([]byte, SALT_BYTES_SIZE+SALT_TIME_SIZE)

	copy(result[SALT_BYTES_START:SALT_BYTES_END], this.GetBytes())
	expTime := this.GetExpirationTime()
	if bytes, err := expTime.MarshalBinary(); err != nil {
		panic(err)
	} else {
		copy(result[SALT_TIME_START:SALT_TIME_END], bytes)
	}

	return result
}
