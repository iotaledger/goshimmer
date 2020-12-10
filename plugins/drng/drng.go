package drng

import (
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/iotaledger/goshimmer/packages/drng"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/logger"
	"github.com/mr-tron/base58/base58"
)

const (
	// Pollen defines the instance ID of the Pollen drng committee.
	Pollen = 1

	// XTeam defines the instance ID of the X-Team drng committee.
	XTeam = 1339

	// Community defines the instance ID of the Community drng committee.
	Community = 7438
)

var (
	// ErrParsingCommitteeMember is returned for an invalid committee member
	ErrParsingCommitteeMember = errors.New("cannot parse committee member")
)

func configureDRNG() *drng.DRNG {
	c := make(map[uint32][]drng.Option)
	log = logger.NewLogger(PluginName)

	// Pollen dRNG configuration
	// parse identities of the committee members
	committeeMembers, err := parseCommitteeMembers(config.Node().Strings(CfgDRNGCommitteeMembers))
	if err != nil {
		log.Warnf("Invalid %s: %s", CfgDRNGCommitteeMembers, err)
	}

	// parse distributed public key of the committee
	dpk, err := parseDistributedPublicKey(CfgDRNGDistributedPubKey)
	if err != nil {
		log.Warn(err)
	}

	// configure pollen committee
	pollenConf := &drng.Committee{
		InstanceID:    Pollen,
		Threshold:     uint8(config.Node().Int(CfgDRNGThreshold)),
		DistributedPK: dpk,
		Identities:    committeeMembers,
	}

	if len(committeeMembers) > 0 {
		c[pollenConf.InstanceID] = []drng.Option{drng.SetCommittee(pollenConf)}
	}

	// X-Team dRNG configuration
	// parse identities of the x-team committee members
	committeeMembers, err = parseCommitteeMembers(config.Node().Strings(CfgDRNGXTeamCommitteeMembers))
	if err != nil {
		log.Warnf("Invalid %s: %s", CfgDRNGXTeamCommitteeMembers, err)
	}

	// parse distributed public key of the committee
	dpk, err = parseDistributedPublicKey(CfgDRNGXTeamDistributedPubKey)
	if err != nil {
		log.Warn(err)
	}

	// configure X-Team committee
	xTeamConf := &drng.Committee{
		InstanceID:    XTeam,
		Threshold:     uint8(config.Node().Int(CfgDRNGXTeamThreshold)),
		DistributedPK: dpk,
		Identities:    committeeMembers,
	}

	if len(committeeMembers) > 0 {
		c[xTeamConf.InstanceID] = []drng.Option{drng.SetCommittee(xTeamConf)}
	}

	// Custom dRNG configuration
	// parse identities of the x-team committee members
	committeeMembers, err = parseCommitteeMembers(config.Node().Strings(CfgDRNGCustomCommitteeMembers))
	if err != nil {
		log.Warnf("Invalid %s: %s", CfgDRNGCustomCommitteeMembers, err)
	}

	// parse distributed public key of the committee
	dpk, err = parseDistributedPublicKey(CfgDRNGCustomDistributedPubKey)
	if err != nil {
		log.Warn(err)
	}

	// configure Custom committee
	customConf := &drng.Committee{
		InstanceID:    uint32(config.Node().Int(CfgDRNGCustomInstanceID)),
		Threshold:     uint8(config.Node().Int(CfgDRNGCustomThreshold)),
		DistributedPK: dpk,
		Identities:    committeeMembers,
	}

	if len(committeeMembers) > 0 {
		if customConf.InstanceID != Pollen && customConf.InstanceID != XTeam {
			c[customConf.InstanceID] = []drng.Option{drng.SetCommittee(customConf)}
		} else {
			log.Warnf("Invalid Custom dRNG instanceID: %d, must be different than both Pollen and X-Team dRNG instance IDs (%d - %d)", customConf.InstanceID, pollenConf.InstanceID, xTeamConf.InstanceID)
		}
	}

	return drng.New(c)
}

// Instance returns the DRNG instance.
func Instance() *drng.DRNG {
	once.Do(func() { instance = configureDRNG() })
	return instance
}

func parseCommitteeMembers(committeeMembers []string) (result []ed25519.PublicKey, err error) {
	for _, committeeMember := range committeeMembers {
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

func parseDistributedPublicKey(pubKey string) (dpk []byte, err error) {
	if str := config.Node().String(pubKey); str != "" {
		dpk, err = hex.DecodeString(str)
		if err != nil {
			return []byte{}, fmt.Errorf("Invalid %s: %s", pubKey, err)
		}
		if l := len(dpk); l != drng.PublicKeySize {
			return []byte{}, fmt.Errorf("Invalid %s length: %d, need %d", pubKey, l, drng.PublicKeySize)
		}
	}
	return
}
