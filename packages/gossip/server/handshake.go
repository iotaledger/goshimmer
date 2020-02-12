package server

import (
	"bytes"
	"time"

	"github.com/golang/protobuf/proto"
	pb "github.com/iotaledger/goshimmer/packages/gossip/server/proto"
	"github.com/iotaledger/hive.go/autopeering/server"
)

const (
	versionNum          = 0
	handshakeExpiration = 20 * time.Second
)

// isExpired checks whether the given UNIX time stamp is too far in the past.
func isExpired(ts int64) bool {
	return time.Since(time.Unix(ts, 0)) >= handshakeExpiration
}

func newHandshakeRequest(toAddr string) ([]byte, error) {
	m := &pb.HandshakeRequest{
		Version:   versionNum,
		To:        toAddr,
		Timestamp: time.Now().Unix(),
	}
	return proto.Marshal(m)
}

func newHandshakeResponse(reqData []byte) ([]byte, error) {
	m := &pb.HandshakeResponse{
		ReqHash: server.PacketHash(reqData),
	}
	return proto.Marshal(m)
}

func (t *TCP) validateHandshakeRequest(reqData []byte) bool {
	m := new(pb.HandshakeRequest)
	if err := proto.Unmarshal(reqData, m); err != nil {
		t.log.Debugw("invalid handshake",
			"err", err,
		)
		return false
	}
	if m.GetVersion() != versionNum {
		t.log.Debugw("invalid handshake",
			"version", m.GetVersion(),
			"want", versionNum,
		)
		return false
	}
	if m.GetTo() != t.publicAddr.String() {
		t.log.Debugw("invalid handshake",
			"to", m.GetTo(),
			"want", t.publicAddr.String(),
		)
		return false
	}
	if isExpired(m.GetTimestamp()) {
		t.log.Debugw("invalid handshake",
			"timestamp", time.Unix(m.GetTimestamp(), 0),
		)
	}

	return true
}

func (t *TCP) validateHandshakeResponse(resData []byte, reqData []byte) bool {
	m := new(pb.HandshakeResponse)
	if err := proto.Unmarshal(resData, m); err != nil {
		t.log.Debugw("invalid handshake",
			"err", err,
		)
		return false
	}
	if !bytes.Equal(m.GetReqHash(), server.PacketHash(reqData)) {
		t.log.Debugw("invalid handshake",
			"hash", m.GetReqHash(),
		)
		return false
	}

	return true
}
