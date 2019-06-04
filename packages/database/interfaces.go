package database

type Database interface {
	Open() error
	Set(key []byte, value []byte) error
	Get(key []byte) ([]byte, error)
	Contains(key []byte) (bool, error)
	Close() error
}
