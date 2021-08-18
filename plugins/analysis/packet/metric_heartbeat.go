package packet

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"

	"github.com/cockroachdb/errors"

	"github.com/iotaledger/hive.go/protocol/message"
	"github.com/iotaledger/hive.go/protocol/tlv"

	"github.com/iotaledger/goshimmer/plugins/banner"
)

var (
	// ErrInvalidMetricHeartbeat is returned for invalid Metric heartbeats.
	ErrInvalidMetricHeartbeat = errors.New("invalid Metric heartbeat")
	// ErrInvalidMetricHeartbeatVersion is returned for invalid Metric heartbeat versions.
	ErrInvalidMetricHeartbeatVersion = errors.New("invalid Metric heartbeat version")
)

// MetricHeartbeatMessageDefinition defines a metric heartbeat message's format.
var MetricHeartbeatMessageDefinition = &message.Definition{
	ID:             MessageTypeMetricHeartbeat,
	MaxBytesLength: 65535,
	VariableLength: true,
}

// MetricHeartbeat represents a metric heartbeat packet.
type MetricHeartbeat struct {
	// The version of GoShimmer.
	Version string
	// The ID of the node who sent the heartbeat.
	// Must be contained when a heartbeat is serialized.
	OwnID []byte
	// OS defines the operating system of the node.
	OS string
	// Arch defines the system architecture of the node.
	Arch string
	// NumCPU defines number of logical cores of the node.
	NumCPU int
	// CPUUsage defines the CPU usage of the node.
	CPUUsage float64
	// MemoryUsage defines the memory usage of the node.
	MemoryUsage uint64
}

// ParseMetricHeartbeat parses a slice of bytes (serialized packet) into a Metric heartbeat.
func ParseMetricHeartbeat(data []byte) (*MetricHeartbeat, error) {
	hb := &MetricHeartbeat{}

	buf := new(bytes.Buffer)
	if _, err := buf.Write(data); err != nil {
		return nil, err
	}

	decoder := gob.NewDecoder(buf)
	if err := decoder.Decode(hb); err != nil {
		return nil, err
	}

	if hb.Version != banner.SimplifiedAppVersion {
		return nil, ErrInvalidMetricHeartbeatVersion
	}

	return hb, nil
}

// Bytes return the Metric heartbeat encoded as bytes.
func (hb MetricHeartbeat) Bytes() ([]byte, error) {
	buf := new(bytes.Buffer)
	encoder := gob.NewEncoder(buf)
	if err := encoder.Encode(hb); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// NewMetricHeartbeatMessage serializes the given Metric heartbeat into a byte slice and adds a TLV header to the packet.
// message = TLV header + serialized packet.
func NewMetricHeartbeatMessage(hb *MetricHeartbeat) ([]byte, error) {
	packet, err := hb.Bytes()
	if err != nil {
		return nil, err
	}

	// calculate total needed bytes based on packet
	packetSize := len(packet)

	// create a buffer for tlv header plus the packet
	buf := bytes.NewBuffer(make([]byte, 0, tlv.HeaderMessageDefinition.MaxBytesLength+uint16(packetSize)))
	// write tlv header into buffer
	if err := tlv.WriteHeader(buf, MessageTypeMetricHeartbeat, uint16(packetSize)); err != nil {
		return nil, err
	}
	// write serialized packet bytes into the buffer
	if err := binary.Write(buf, binary.BigEndian, packet); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
