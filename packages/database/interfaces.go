package database

import "time"

type Database interface {
	Set(key []byte, value []byte) error
	SetWithTTL(key []byte, value []byte, ttl time.Duration) error
	Contains(key []byte) (bool, error)
	Get(key []byte) ([]byte, error)
	ForEach(func(key []byte, value []byte)) error
	Delete(key []byte) error
}
