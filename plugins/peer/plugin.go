package peer

import (
	"bytes"
	"context"
	"encoding/base64"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/mr-tron/base58"
	"github.com/pkg/errors"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/core/database"
	"github.com/iotaledger/goshimmer/packages/core/shutdown"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/plugins/protocol"
	"github.com/iotaledger/hive.go/app/daemon"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/kvstore"
)

// PluginName is the name of the Peer plugin.
const PluginName = "Peer"

var (
	// Plugin is the plugin instance of the Peer plugin.
	Plugin *node.Plugin

	// ErrMismatchedPrivateKeys is returned when the private key derived from the config does not correspond to the private
	// key stored in an already existing peer database.
	ErrMismatchedPrivateKeys = errors.New("private key derived from the seed defined in the config does not correspond with the already stored private key in the database")

	deps = new(dependencies)
)

type dependencies struct {
	dig.In

	PeerDB        *peer.DB
	PeerDBKVSTore kvstore.KVStore `name:"peerDBKVStore"`
}

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Enabled, run)

	Plugin.Events.Init.Hook(func(e *node.InitEvent) {
		if err := e.Container.Provide(configureLocalPeer); err != nil {
			Plugin.Panic(err)
		}
	})
}

func run(_ *node.Plugin) {
	if err := daemon.BackgroundWorker(PluginName, func(ctx context.Context) {
		<-ctx.Done()
		prvKey, _ := deps.PeerDB.LocalPrivateKey()
		if err := deps.PeerDBKVSTore.Close(); err != nil {
			Plugin.Logger().Errorf("unable to save identity %s: %s", prvKey.Public().String(), err)
			return
		}
		Plugin.Logger().Infof("saved identity %s", prvKey.Public().String())
	}, shutdown.PriorityPeerDatabase); err != nil {
		Plugin.Logger().Fatalf("Failed to start as daemon: %s", err)
	}
}

type peerOut struct {
	dig.Out
	Peer          *peer.Local
	PeerDB        *peer.DB
	PeerDBKVSTore kvstore.KVStore `name:"peerDBKVStore"`
}

// instantiates a local instance.
func configureLocalPeer() peerOut {
	peerDB, peerDBKVStore, isNewDB, err := initPeerDB()
	if err != nil {
		Plugin.Logger().Fatal(err)
	}

	var seed [][]byte
	cfgSeedSet := Parameters.Seed != ""
	if cfgSeedSet {
		readSeed, cfgReadErr := readSeedFromCfg()
		if cfgReadErr != nil {
			Plugin.Logger().Fatal(cfgReadErr)
		}
		seed = append(seed, readSeed)
	}

	if !isNewDB && cfgSeedSet && !Parameters.OverwriteStoredSeed {
		if seedCheckErr := checkCfgSeedAgainstDB(seed[0], peerDB); seedCheckErr != nil {
			Plugin.Logger().Fatal(seedCheckErr)
		}
	}

	peeringIP, err := readPeerIP()
	if err != nil {
		Plugin.Logger().Fatal(err)
	}

	if !peeringIP.IsGlobalUnicast() {
		Plugin.Logger().Warnf("IP is not a global unicast address: %s", peeringIP.String())
	}

	// TODO: remove requirement for PeeringKey in hive.go
	services := service.New()
	services.Update(service.PeeringKey, "dummy", 0)

	local, err := peer.NewLocal(peeringIP, services, peerDB, seed...)
	if err != nil {
		Plugin.Logger().Fatalf("Error creating local: %s", err)
	}

	Plugin.Logger().Infof("Initialized local: %v", local)

	return peerOut{
		Peer:          local,
		PeerDB:        peerDB,
		PeerDBKVSTore: peerDBKVStore,
	}
}

// checks whether the seed from the cfg corresponds to the one in the peer database.
func checkCfgSeedAgainstDB(cfgSeed []byte, peerDB *peer.DB) error {
	prvKeyDB, err := peerDB.LocalPrivateKey()
	if err != nil {
		return errors.Wrapf(err, "unable to retrieve private key from peer database")
	}
	prvKeyDBBytes, err := prvKeyDB.Bytes()
	if err != nil {
		return err
	}
	prvKeyCfg := ed25519.PrivateKeyFromSeed(cfgSeed)
	prvKeyCfgBytes, err := prvKeyCfg.Bytes()
	if err != nil {
		return err
	}

	if !bytes.Equal(prvKeyCfgBytes, prvKeyDBBytes) {
		return errors.WithMessagef(ErrMismatchedPrivateKeys, "identities - pub keys (cfg/db): %s vs. %s", prvKeyCfg.Public().String(), prvKeyDB.Public().String())
	}

	return nil
}

func readPeerIP() (net.IP, error) {
	if strings.ToLower(Parameters.ExternalAddress) == "auto" {
		// let the autopeering discover the IP
		return net.IPv4zero, nil
	}

	peeringIP := net.ParseIP(Parameters.ExternalAddress)
	if peeringIP == nil {
		return nil, errors.Errorf("invalid IP address: %s", Parameters.ExternalAddress)
	}

	return peeringIP, nil
}

// inits the peer database, returns a bool indicating whether the database is new.
func initPeerDB() (peerDB *peer.DB, peerDBKVStore kvstore.KVStore, isNewDB bool, err error) {
	if err = checkValidPeerDBPath(); err != nil {
		return nil, nil, false, errors.Wrap(err, "invalid peer database path")
	}

	if isNewDB, err = isPeerDBNew(); err != nil {
		return nil, nil, false, errors.Wrap(err, "unable to check whether peer database is new")
	}

	db, err := database.NewDB(Parameters.PeerDBDirectory)
	if err != nil {
		return nil, nil, false, errors.Wrap(err, "error creating peer database")
	}

	if peerDBKVStore, err = db.NewStore().WithExtendedRealm([]byte{database.PrefixPeer}); err != nil {
		return nil, nil, false, errors.Wrap(err, "error creating peer store")
	}

	if peerDB, err = peer.NewDB(peerDBKVStore); err != nil {
		return nil, nil, false, errors.Wrap(err, "error creating peer database")
	}

	if db == nil {
		return nil, nil, false, errors.New("couldn't create peer database; nil")
	}

	return
}

// checks whether the peer database is new by examining whether the directory
// exists or whether it contains any files.
func isPeerDBNew() (bool, error) {
	var isNewDB bool
	fileInfo, err := os.Stat(Parameters.PeerDBDirectory)
	switch {
	case fileInfo != nil:
		files, readDirErr := os.ReadDir(Parameters.PeerDBDirectory)
		if readDirErr != nil {
			return false, errors.Wrap(readDirErr, "unable to check whether peer database is empty")
		}
		if len(files) != 0 {
			break
		}
		fallthrough
	case os.IsNotExist(err):
		isNewDB = true
	}

	return isNewDB, nil
}

// checks that the peer database path does not reside within the main database directory.
func checkValidPeerDBPath() error {
	absMainDBPath, err := filepath.Abs(protocol.DatabaseParameters.Directory)
	if err != nil {
		return errors.Wrapf(err, "cannot resolve absolute path of %s", protocol.DatabaseParameters.Directory)
	}

	absPeerDBPath, err := filepath.Abs(Parameters.PeerDBDirectory)
	if err != nil {
		return errors.Wrapf(err, "cannot resolve absolute path of %s", Parameters.PeerDBDirectory)
	}

	if strings.Index(absPeerDBPath, absMainDBPath) == 0 {
		return errors.Errorf("peerDB: %s should not be a subdirectory of mainDB: %s", Parameters.PeerDBDirectory, protocol.DatabaseParameters.Directory)
	}
	return nil
}

func readSeedFromCfg() ([]byte, error) {
	var seedBytes []byte
	var err error

	switch {
	case strings.HasPrefix(Parameters.Seed, "base58:"):
		seedBytes, err = base58.Decode(Parameters.Seed[7:])
	case strings.HasPrefix(Parameters.Seed, "base64:"):
		seedBytes, err = base64.StdEncoding.DecodeString(Parameters.Seed[7:])
	default:
		err = errors.New("neither base58 nor base64 prefix provided")
	}

	if err != nil {
		return nil, errors.Wrap(err, "invalid seed")
	}

	if l := len(seedBytes); l != ed25519.SeedSize {
		return nil, errors.Errorf("invalid seed length: %d, need %d", l, ed25519.SeedSize)
	}

	return seedBytes, nil
}
