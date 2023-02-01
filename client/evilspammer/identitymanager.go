package evilspammer

import (
	"github.com/iotaledger/hive.go/core/crypto/ed25519"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/mr-tron/base58"
)

type IdentityManager struct {
	identities   map[string]*identity.LocalIdentity
	log          Logger
	primaryAlias string
}

func NewIdentityManager() *IdentityManager {
	return &IdentityManager{
		identities: make(map[string]*identity.LocalIdentity),
	}
}

func (i *IdentityManager) SetLogger(log Logger) {
	i.log = log
}

func (i *IdentityManager) AddIdentity(privateKey, aliasName string) {
	if privateKey == "" {
		panic("private key is empty")
	}
	seedBytes, err := base58.Decode(privateKey)
	if err != nil {
		panic(err)
	}
	key := ed25519.PrivateKeyFromSeed(seedBytes)
	local := identity.NewLocalIdentity(key.Public(), key)
	i.identities[aliasName] = local
}

func (i *IdentityManager) GetIdentity() *identity.LocalIdentity {
	return i.getIdentity(i.primaryAlias)
}

func (i *IdentityManager) getIdentity(alias string) *identity.LocalIdentity {
	id, exists := i.identities[alias]

	if !exists {
		i.log.Warnf("Identity %s does not exist, generating new one", alias)
		key := ed25519.GenerateKeyPair().PrivateKey
		return identity.NewLocalIdentity(key.Public(), key)
	}
	return id
}
