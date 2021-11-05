package drng

import (
	"encoding/hex"
	"fmt"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/mr-tron/base58/base58"

	"github.com/iotaledger/goshimmer/packages/drng"
)

const (
	// Pollen defines the instance ID of the Pollen drng committee.
	Pollen = 1

	// XTeam defines the instance ID of the X-Team drng committee.
	XTeam = 1339

	// Community defines the instance ID of the Community drng committee.
	Community = 7438
)

// ErrParsingCommitteeMember is returned for an invalid committee member
var ErrParsingCommitteeMember = errors.New("cannot parse committee member")

func configureDRNG() *drng.DRNG {
	c := make(map[uint32][]drng.Option)

	// Pollen dRNG configuration
	// parse identities of the committee members
	committeeMembers, err := parseCommitteeMembers(Parameters.Pollen.CommitteeMembers)
	if err != nil {
		Plugin.LogWarnf("Invalid Pollen committee members: %s", err)
	}

	// parse distributed public key of the committee
	dpk, err := parseDistributedPublicKey(Parameters.Pollen.DistributedPubKey)
	if err != nil {
		Plugin.LogWarn(err)
	}

	// configure Pollen committee
	pollenConf := &drng.Committee{
		InstanceID:    Pollen,
		Threshold:     uint8(Parameters.Pollen.Threshold),
		DistributedPK: dpk,
		Identities:    committeeMembers,
	}

	if len(committeeMembers) > 0 {
		c[pollenConf.InstanceID] = []drng.Option{drng.SetCommittee(pollenConf)}
	}

	// X-Team dRNG configuration
	// parse identities of the x-team committee members
	committeeMembers, err = parseCommitteeMembers(Parameters.XTeam.CommitteeMembers)
	if err != nil {
		Plugin.LogWarnf("Invalid X-Team committee members: %s", err)
	}

	// parse distributed public key of the committee
	dpk, err = parseDistributedPublicKey(Parameters.XTeam.DistributedPubKey)
	if err != nil {
		Plugin.LogWarn(err)
	}

	// configure X-Team committee
	xTeamConf := &drng.Committee{
		InstanceID:    XTeam,
		Threshold:     uint8(Parameters.XTeam.Threshold),
		DistributedPK: dpk,
		Identities:    committeeMembers,
	}

	if len(committeeMembers) > 0 {
		c[xTeamConf.InstanceID] = []drng.Option{drng.SetCommittee(xTeamConf)}
	}

	// Custom dRNG configuration
	// parse identities of the x-team committee members
	committeeMembers, err = parseCommitteeMembers(Parameters.Custom.CommitteeMembers)
	if err != nil {
		Plugin.LogWarn("Invalid custom committee members: %s", err)
	}

	// parse distributed public key of the committee
	dpk, err = parseDistributedPublicKey(Parameters.Custom.DistributedPubKey)
	if err != nil {
		Plugin.LogWarn(err)
	}

	// configure Custom committee
	customConf := &drng.Committee{
		InstanceID:    uint32(Parameters.Custom.InstanceID),
		Threshold:     uint8(Parameters.Custom.Threshold),
		DistributedPK: dpk,
		Identities:    committeeMembers,
	}

	if len(committeeMembers) > 0 {
		if customConf.InstanceID != Pollen && customConf.InstanceID != XTeam {
			c[customConf.InstanceID] = []drng.Option{drng.SetCommittee(customConf)}
		} else {
			Plugin.LogWarnf("Invalid Custom dRNG instanceID: %d, must be different than both Pollen and X-Team dRNG instance IDs (%d - %d)", customConf.InstanceID, pollenConf.InstanceID, xTeamConf.InstanceID)
		}
	}

	return drng.New(c)
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
	if pubKey != "" {
		dpk, err = hex.DecodeString(pubKey)
		if err != nil {
			return []byte{}, fmt.Errorf("invalid %s: %s", pubKey, err)
		}
		if l := len(dpk); l != drng.PublicKeySize {
			return []byte{}, fmt.Errorf("invalid %s length: %d, need %d", pubKey, l, drng.PublicKeySize)
		}
	}
	return
}
