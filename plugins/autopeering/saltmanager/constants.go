package saltmanager

import "time"

const (
	PUBLIC_SALT_LIFETIME  = 1800 * time.Second
	PRIVATE_SALT_LIFETIME = 1800 * time.Second
)

var (
	PUBLIC_SALT_SETTINGS_KEY  = []byte("PUBLIC_SALT")
	PRIVATE_SALT_SETTINGS_KEY = []byte("PRIVATE_SALT")
)
