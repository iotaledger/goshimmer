package peer

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/mr-tron/base58"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/plugins/database"

	databasePkg "github.com/iotaledger/goshimmer/packages/database"
)

// PluginName is the name of the Peer plugin.
const PluginName = "Peer"

var (
	// Plugin is the plugin instance of the Peer plugin.
	Plugin *node.Plugin

	// ErrMismatchedPrivateKeys is returned when the private key derived from the config does not correspond to the private
	// key stored in an already existing peer database.
	ErrMismatchedPrivateKeys = errors.New("private key derived from the seed defined in the config does not correspond with the already stored private key in the database")
)

func init() {
	Plugin = node.NewPlugin(PluginName, nil, node.Enabled)

	Plugin.Events.Init.Attach(events.NewClosure(func(_ *node.Plugin, container *dig.Container) {
		if err := container.Provide(configureLocal); err != nil {
			Plugin.Panic(err)
		}
	}))
}

// instantiates a local instance.
func configureLocal() *peer.Local {
	log := logger.NewLogger("Local")

	peeringIP, err := readPeerIP()
	if err != nil {
		log.Fatal(err)
	}

	if !peeringIP.IsGlobalUnicast() {
		log.Warnf("IP is not a global unicast address: %s", peeringIP.String())
	}

	var seed [][]byte
	cfgSeedSet := Parameters.Seed != ""
	if cfgSeedSet {
		readSeed, err := readSeedFromCfg()
		if err != nil {
			log.Fatal(err)
		}
		seed = append(seed, readSeed)
	}

	peerDB, isNewDB, err := initPeerDB()
	if err != nil {
		log.Fatal(err)
	}

	if !isNewDB && cfgSeedSet {
		if err := checkCfgSeedAgainstDB(seed[0], peerDB); err != nil {
			log.Fatal(err)
		}
	}

	// TODO: remove requirement for PeeringKey in hive.go
	services := service.New()
	services.Update(service.PeeringKey, "dummy", 0)

	local, err := peer.NewLocal(peeringIP, services, peerDB, seed...)
	if err != nil {
		log.Fatalf("Error creating local: %s", err)
	}
	log.Infof("Initialized local: %v", local)

	return local
}

// checks whether the seed from the cfg corresponds to the one in the peer database.
func checkCfgSeedAgainstDB(cfgSeed []byte, peerDB *peer.DB) error {
	prvKeyDB, err := peerDB.LocalPrivateKey()
	if err != nil {
		return fmt.Errorf("unable to retrieve private key from peer database: %w", err)
	}
	prvKeyCfg := ed25519.PrivateKeyFromSeed(cfgSeed)
	if !bytes.Equal(prvKeyCfg.Bytes(), prvKeyDB.Bytes()) {
		return fmt.Errorf("%w: identities - pub keys (cfg/db): %s vs. %s", ErrMismatchedPrivateKeys, prvKeyCfg.Public().String(), prvKeyDB.Public().String())
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
		return nil, fmt.Errorf("invalid IP address: %s", Parameters.ExternalAddress)
	}

	return peeringIP, nil
}

// inits the peer database, returns a bool indicating whether the database is new.
func initPeerDB() (*peer.DB, bool, error) {
	if err := checkValidPeerDBPath(); err != nil {
		return nil, false, err
	}

	isNewDB, err := isPeerDBNew()
	if err != nil {
		return nil, false, err
	}

	db, err := databasePkg.NewDB(Parameters.PeerDBDirectory)
	if err != nil {
		return nil, false, fmt.Errorf("error creating DB: %s", err)
	}

	peerDB, err := peer.NewDB(db.NewStore().WithRealm([]byte{databasePkg.PrefixPeer}))
	if err != nil {
		return nil, false, fmt.Errorf("error creating peer DB: %w", err)
	}
	if db == nil {
		return nil, false, fmt.Errorf("couldn't create peerDB; nil")
	}

	return peerDB, isNewDB, nil
}

// checks whether the peer database is new by examining whether the directory
// exists or whether it contains any files.
func isPeerDBNew() (bool, error) {
	var isNewDB bool
	fileInfo, err := os.Stat(Parameters.PeerDBDirectory)
	switch {
	case fileInfo != nil:
		files, err := os.ReadDir(Parameters.PeerDBDirectory)
		if err != nil {
			return false, fmt.Errorf("unable to check whether peer database is empty: %w", err)
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
	absMainDBPath, err := filepath.Abs(database.Parameters.Directory)
	if err != nil {
		return fmt.Errorf("cannot resolve absolute path of %s: %w", database.Parameters.Directory, err)
	}

	absPeerDBPath, err := filepath.Abs(Parameters.PeerDBDirectory)
	if err != nil {
		return fmt.Errorf("cannot resolve absolute path of %s: %w", Parameters.PeerDBDirectory, err)
	}

	if strings.Index(absPeerDBPath, absMainDBPath) == 0 {
		return fmt.Errorf("peerDB: %s should not be a subdirectory of mainDB: %s", Parameters.PeerDBDirectory, database.Parameters.Directory)
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
		err = fmt.Errorf("neither base58 nor base64 prefix provided")
	}

	if err != nil {
		return nil, fmt.Errorf("invalid seed: %w", err)
	}

	if l := len(seedBytes); l != ed25519.SeedSize {
		return nil, fmt.Errorf("invalid seed length: %d, need %d", l, ed25519.SeedSize)
	}

	return seedBytes, nil
}
