package snapshotcreator

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/core/database"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/runtime/module"
	"github.com/iotaledger/hive.go/runtime/options"

	"github.com/mr-tron/base58/base58"
)

// Options stores the details about snapshots created for integration tests
type Options struct {
	// FilePath is the path to the snapshot file.
	FilePath string
	// GenesisSeed is the seed of the PeerMaster node where the genesis pledge goes to.
	GenesisSeed []byte
	// GenesisTokenAmount is the amount of tokens left on the Genesis, pledged to Peer Master.
	GenesisTokenAmount uint64
	// TotalTokensPledged is the total amount of tokens pledged from genesis to the peers.
	// If provided mana will be distributed equally.
	TotalTokensPledged uint64
	// PeersSeedBase58 is a slice of Seeds encoded in Base58, one entry per peer.
	PeersSeedBase58 []string
	// PeersPublicKey is a slice of public keys, one entry per peer.
	PeersPublicKey []ed25519.PublicKey
	// PeersAmountsPledged is a slice of amounts to be pledged to the peers, one entry per peer.
	PeersAmountsPledged []uint64
	// InitialAttestationsPublicKey indicates which node public key should be included in the first commitment.
	InitialAttestationsPublicKey []ed25519.PublicKey
	// AttestAll indicates that all nodes will be included in the attestation.
	AttestAll bool
	// GenesisUnixTime provides the genesis time of the snapshot.
	GenesisUnixTime int64
	// SlotDuration defines the duration in seconds of each slot.
	SlotDuration int64

	DataBaseVersion database.Version

	LedgerProvider module.Provider[*engine.Engine, ledger.Ledger]
}

func NewOptions(opts ...options.Option[Options]) *Options {
	return options.Apply(&Options{
		FilePath:        "snapshot.bin",
		DataBaseVersion: 1,
		GenesisUnixTime: time.Now().Unix(),
		SlotDuration:    10,
	}, opts)
}

func WithFilePath(filePath string) options.Option[Options] {
	return func(m *Options) {
		m.FilePath = filePath
	}
}

func WithGenesisSeed(masterSeed []byte) options.Option[Options] {
	return func(m *Options) {
		m.GenesisSeed = masterSeed
	}
}

// WithGenesisTokenAmount sets the amount of tokens left on the Genesis, pledged to Peer Master.
func WithGenesisTokenAmount(genesisTokenAmount uint64) options.Option[Options] {
	return func(m *Options) {
		m.GenesisTokenAmount = genesisTokenAmount
	}
}

// WithTotalTokensPledged sets the total amount of tokens pledged from genesis to the peers,
// if not provided all genesis tokens will be distributed equally.
func WithTotalTokensPledged(totalTokensPledged uint64) options.Option[Options] {
	return func(m *Options) {
		m.TotalTokensPledged = totalTokensPledged
	}
}

// WithPeersSeedBase58 sets the seed of the peers to be used in the snapshot.
func WithPeersSeedBase58(peersSeedBase58 []string) options.Option[Options] {
	return func(m *Options) {
		m.PeersPublicKey = make([]ed25519.PublicKey, len(peersSeedBase58))
		for i, seed58 := range peersSeedBase58 {
			b, err := base58.Decode(seed58)
			if err != nil {
				panic("failed to decode peer seed: " + err.Error())
			}
			nodePublicKey := ed25519.PrivateKeyFromSeed(b).Public()
			m.PeersPublicKey[i] = nodePublicKey
		}
		m.PeersSeedBase58 = peersSeedBase58
	}
}

// WithPeersPublicKeysBase58 sets the public keys of the peers based on provided base58 encoded public keys.
func WithPeersPublicKeysBase58(peersPublicKeyBase58 []string) options.Option[Options] {
	return func(m *Options) {
		m.PeersPublicKey = make([]ed25519.PublicKey, len(peersPublicKeyBase58))
		for i, pk := range peersPublicKeyBase58 {
			b, err := base58.Decode(pk)
			if err != nil {
				panic("failed to decode peer seed: " + err.Error())
			}
			nodePublicKey, _, err := ed25519.PublicKeyFromBytes(b)
			if err != nil {
				panic("failed to read public key from bytes: " + err.Error())
			}
			m.PeersPublicKey[i] = nodePublicKey
		}
	}
}

func WithPeersPublicKeys(peersPublicKey []ed25519.PublicKey) options.Option[Options] {
	return func(m *Options) {
		m.PeersPublicKey = peersPublicKey
	}
}

// WithPeersAmountsPledged sets the amount of tokens to be pledged to the peers.
func WithPeersAmountsPledged(peersAmountsPledged []uint64) options.Option[Options] {
	return func(m *Options) {
		m.PeersAmountsPledged = peersAmountsPledged
	}
}

// WithPledgeIDs sets the public keys and pledge amounts to use for the snapshot.
func WithPledgeIDs(pledgeIDs map[ed25519.PublicKey]uint64) options.Option[Options] {
	return func(m *Options) {
		m.PeersPublicKey, m.PeersAmountsPledged = KeyValues(pledgeIDs)
	}
}

// WithInitialAttestationsBase58 sets the initial attestation node to use for the snapshot based on provided public keys in base58.
func WithInitialAttestationsBase58(initialAttestationBase58 []string) options.Option[Options] {
	return func(m *Options) {
		m.InitialAttestationsPublicKey = make([]ed25519.PublicKey, len(initialAttestationBase58))
		for i, attestationBase58 := range initialAttestationBase58 {
			b, err := base58.Decode(attestationBase58)
			if err != nil {
				panic("failed to decode attestation: " + err.Error())
			}
			nodePublicKey, _, err := ed25519.PublicKeyFromBytes(b)
			if err != nil {
				panic("failed to read public key from bytes: " + err.Error())
			}
			m.InitialAttestationsPublicKey[i] = nodePublicKey
		}
	}
}

// WithInitialAttestationsPublicKey sets the initial attestation node public key to use for the snapshot.
func WithInitialAttestationsPublicKey(initialAttestation []ed25519.PublicKey) options.Option[Options] {
	return func(m *Options) {
		m.InitialAttestationsPublicKey = initialAttestation
	}
}

// WithAttestAll indicates if all node should be included in attestations.
func WithAttestAll(attestAll bool) options.Option[Options] {
	return func(m *Options) {
		m.AttestAll = attestAll
	}
}

// WithDatabaseVersion sets the database version to use for the snapshot.
func WithDatabaseVersion(databaseVersion database.Version) options.Option[Options] {
	return func(m *Options) {
		m.DataBaseVersion = databaseVersion
	}
}

// WithLedgerProvider sets the MemPool to use for the snapshot.
func WithLedgerProvider(ledgerProvider module.Provider[*engine.Engine, ledger.Ledger]) options.Option[Options] {
	return func(m *Options) {
		m.LedgerProvider = ledgerProvider
	}
}

// WithGenesisUnixTime provides the genesis time of the snapshot.
func WithGenesisUnixTime(unixTime int64) options.Option[Options] {
	return func(m *Options) {
		m.GenesisUnixTime = unixTime
	}
}

// WithSlotDuration defines the duration in seconds of each slot.
func WithSlotDuration(duration int64) options.Option[Options] {
	return func(m *Options) {
		m.SlotDuration = duration
	}
}

func KeyValues[K comparable, V any](in map[K]V) ([]K, []V) {
	keys := make([]K, 0, len(in))
	values := make([]V, 0, len(in))

	for k, v := range in {
		keys = append(keys, k)
		values = append(values, v)
	}

	return keys, values
}
