package drng

import (
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/iotaledger/goshimmer/packages/binary/drng"
	"github.com/iotaledger/goshimmer/packages/binary/drng/state"
	cbPayload "github.com/iotaledger/goshimmer/packages/binary/drng/subtypes/collectiveBeacon/payload"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/logger"
	"github.com/mr-tron/base58/base58"
)

var (
	// ErrParsingCommitteeMember is returned for an invalid committee member
	ErrParsingCommitteeMember = errors.New("cannot parse committee member")
)

func configureDRNG() *drng.DRNG {
	log = logger.NewLogger(PluginName)
	// parse identities of the committee members
	committeeMembers, err := parseCommitteeMembers()
	if err != nil {
		log.Warnf("Invalid %s: %s", CfgDRNGCommitteeMembers, err)
	}

	// parse distributed public key of the committee
	var dpk []byte
	if str := config.Node.GetString(CfgDRNGDistributedPubKey); str != "" {
		bytes, err := hex.DecodeString(str)
		if err != nil {
			log.Warnf("Invalid %s: %s", CfgDRNGDistributedPubKey, err)
		}
		if l := len(bytes); l != cbPayload.PublicKeySize {
			log.Warnf("Invalid %s length: %d, need %d", CfgDRNGDistributedPubKey, l, cbPayload.PublicKeySize)
		}
		dpk = append(dpk, bytes...)
	}

	// configure committee
	committeeConf := &state.Committee{
		InstanceID:    config.Node.GetUint32(CfgDRNGInstanceID),
		Threshold:     uint8(config.Node.GetUint32(CfgDRNGThreshold)),
		DistributedPK: dpk,
		Identities:    committeeMembers,
	}

	return drng.New(state.SetCommittee(committeeConf))
}

// Instance returns the DRNG instance.
func Instance() *drng.DRNG {
	once.Do(func() { instance = configureDRNG() })
	return instance
}

func parseCommitteeMembers() (result []ed25519.PublicKey, err error) {
	for _, committeeMember := range config.Node.GetStringSlice(CfgDRNGCommitteeMembers) {
		if committeeMember == "" {
			continue
		}

		pubKey, err := base58.Decode(committeeMember)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid public key: %s", ErrParsingCommitteeMember, err)
		}
		publicKey, _, err := ed25519.PublicKeyFromBytes(pubKey)
		if err != nil {
			return nil, err
		}

		result = append(result, publicKey)
	}

	return result, nil
}
