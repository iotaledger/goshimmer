package snapshotcreator

import (
	"github.com/iotaledger/goshimmer/packages/core/database"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/vm"
	"github.com/iotaledger/hive.go/core/crypto/ed25519"
	"github.com/iotaledger/hive.go/core/generics/options"
)

// Options stores the details about snapshots created for integration tests
type Options struct {
	FilePath string
	// GenesisSeed is the seed of the PeerMaster node where the genesis pledge goes to.
	GenesisSeed []byte
	// GenesisTokenAmount is the amount of tokens left on the Genesis, pledged to Peer Master.
	GenesisTokenAmount uint64
	// TotalTokensPledged is the total amount of tokens pledged from genesis to the peers.
	// If provided mana will be distributed equally.
	TotalTokensPledged uint64
	// PeerSeedBase58 is a slice of Seeds encoded in Base58, one entry per peer.
	PeersSeedBase58 []string
	// PeersPublicKey is a slice of public keys, one entry per peer.
	PeersPublicKey []ed25519.PublicKey
	// PeersAmountsPledges is a slice of amounts to be pledged to the peers, one entry per peer.
	PeersAmountsPledged []uint64
	// InitialAttestation indicates which node public key should be included in the first commitment.
	InitialAttestation string
	// StartSynced indicates that all nodes will be included in the attestation.
	StartSynced bool

	dataBaseVersion database.Version
	vm              vm.VM
}

func NewOptions(opts ...options.Option[Options]) *Options {
	return options.Apply(&Options{
		FilePath: "snapshot.bin",
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

// WithPeersSeedBase58 sets the seed of the peers to be used in the snapshot.
func WithPeersSeedBase58(peersSeedBase58 []string) options.Option[Options] {
	return func(m *Options) {
		m.PeersSeedBase58 = peersSeedBase58
	}
}

func WithPeersPublicKey(peersPublicKey []ed25519.PublicKey) options.Option[Options] {
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

// WithInitialAttestation sets the initial attestation node to use for the snapshot.
func WithInitialAttestation(initialAttestation string) options.Option[Options] {
	return func(m *Options) {
		m.InitialAttestation = initialAttestation
	}
}

// WithStartSynced indicates if all node should be included in attestations.
func WithStartSynced(startSynced bool) options.Option[Options] {
	return func(m *Options) {
		m.StartSynced = startSynced
	}
}

// WithDatabaseVersion sets the database version to use for the snapshot.
func WithDatabaseVersion(databaseVersion database.Version) options.Option[Options] {
	return func(m *Options) {
		m.dataBaseVersion = databaseVersion
	}
}

// WithVM sets the VM to use for the snapshot.
func WithVM(vm vm.VM) options.Option[Options] {
	return func(m *Options) {
		m.vm = vm
	}
}
