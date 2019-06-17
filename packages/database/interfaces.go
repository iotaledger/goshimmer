package database

type Database interface {
	Open() error
	Set(key []byte, value []byte) error
	Contains(key []byte) (bool, error)
	Get(key []byte) ([]byte, error)
	ForEach(func(key []byte, value []byte)) error
	Delete(key []byte) error
	Close() error
}
