package drng

import (
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/hive.go/crypto/ed25519"
)

var (
	// ErrParsingCommitteeMember is returned for an invalid committee member
	ErrParsingCommitteeMember = errors.New("cannot parse committee member")
)

func parseCommitteeMembers() (result []ed25519.PublicKey, err error) {
	for _, committeeMember := range config.Node.GetStringSlice(CfgDRNGCommitteeMembers) {
		if committeeMember == "" {
			continue
		}

		pubKey, err := hex.DecodeString(committeeMember)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid public key: %s", ErrParsingCommitteeMember, err)
		}
		publicKey, err, _ := ed25519.PublicKeyFromBytes(pubKey)
		if err != nil {
			return nil, err
		}

		result = append(result, publicKey)
	}

	return result, nil
}
