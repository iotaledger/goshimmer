package packet

import (
	"crypto/sha256"
	"runtime"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/protocol/message"
	"github.com/iotaledger/hive.go/protocol/tlv"
	"github.com/shirou/gopsutil/cpu"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/plugins/banner"
)

var nodeID = sha256.Sum256([]byte{'A'})

func testMetricHeartbeat() *MetricHeartbeat {
	return &MetricHeartbeat{
		OwnID:  nodeID[:],
		OS:     runtime.GOOS,
		Arch:   runtime.GOARCH,
		NumCPU: runtime.GOMAXPROCS(0),
		CPUUsage: func() (p float64) {
			percent, err := cpu.Percent(time.Second, false)
			if err == nil {
				p = percent[0]
			}
			return
		}(),
		MemoryUsage: func() uint64 {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			return m.Alloc
		}(),
	}
}

func TestMetricHeartbeat(t *testing.T) {
	hb := testMetricHeartbeat()

	packet, err := hb.Bytes()
	require.NoError(t, err)

	_, err = ParseMetricHeartbeat(packet)
	require.Error(t, err)

	hb.Version = banner.SimplifiedAppVersion
	packet, err = hb.Bytes()
	require.NoError(t, err)

	hbParsed, err := ParseMetricHeartbeat(packet)
	require.NoError(t, err)

	require.Equal(t, hb, hbParsed)

	tlvHeaderLength := int(tlv.HeaderMessageDefinition.MaxBytesLength)
	msg, err := NewMetricHeartbeatMessage(hb)
	require.NoError(t, err)

	require.Equal(t, MessageTypeMetricHeartbeat, message.Type(msg[0]))

	hbParsed, err = ParseMetricHeartbeat(msg[tlvHeaderLength:])
	require.NoError(t, err)
	require.Equal(t, hb, hbParsed)
}
