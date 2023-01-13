package dashboard

import (
	"sync"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/throughputquota/mana1/manamodels"
)

const (
	// the age of the oldest event depends on number of utxo inputs spent recently
	// note, that this is aggregate for both access and consensus events.
	maxManaEventsBufferSize = 200
	// oldest data is 10s * 100 = 17 mins before dropping it.
	maxManaValuesBufferSize = 100
)

// ManaBuffer holds recent data related to mana in the dashboard. Used to fill frontend on page load/reload.
type ManaBuffer struct {
	// Events store PledgedEvent and RevokedEvent structs in chronological order.
	eventsMutex     sync.RWMutex
	ValueBlks       []*ManaValueBlkData
	valueBlksMutex  sync.RWMutex
	MapOverall      map[manamodels.Type]*ManaNetworkListBlkData
	mapOverallMutex sync.RWMutex
	MapOnline       map[manamodels.Type]*ManaNetworkListBlkData
	mapOnlineMutex  sync.RWMutex
}

// NewManaBuffer creates and initializes a new, empty buffer.
func NewManaBuffer() *ManaBuffer {
	return &ManaBuffer{
		ValueBlks:  make([]*ManaValueBlkData, 0),
		MapOverall: make(map[manamodels.Type]*ManaNetworkListBlkData),
		MapOnline:  make(map[manamodels.Type]*ManaNetworkListBlkData),
	}
}

// StoreValueBlk stores a value blk in the buffer. If it is full, drops the oldest blk.
func (m *ManaBuffer) StoreValueBlk(blk *ManaValueBlkData) {
	m.valueBlksMutex.Lock()
	defer m.valueBlksMutex.Unlock()
	if len(m.ValueBlks) >= maxManaValuesBufferSize {
		// drop oldest blk if buffer is full
		m.ValueBlks = m.ValueBlks[1:]
	}
	m.ValueBlks = append(m.ValueBlks, blk)
}

// SendValueBlks sends all blks in the buffer through the provided websocket connection.
func (m *ManaBuffer) SendValueBlks(ws *websocket.Conn) error {
	m.valueBlksMutex.RLock()
	defer m.valueBlksMutex.RUnlock()
	for _, valueBlk := range m.ValueBlks {
		blk := &wsblk{
			Type: MsgTypeManaValue,
			Data: valueBlk,
		}
		if err := sendJSON(ws, blk); err != nil {
			return errors.Wrap(err, "failed to send mana value to client")
		}
	}
	return nil
}

// StoreMapOverall stores network mana map blk data.
func (m *ManaBuffer) StoreMapOverall(blks ...*ManaNetworkListBlkData) {
	m.mapOverallMutex.Lock()
	defer m.mapOverallMutex.Unlock()
	for _, blk := range blks {
		manaType, err := manamodels.TypeFromString(blk.ManaType)
		if err != nil {
			log.Errorf("couldn't parse type of mana: %w", err)
			continue
		}
		m.MapOverall[manaType] = blk
	}
}

// SendMapOverall sends buffered overall mana maps to the provided websocket connection.
func (m *ManaBuffer) SendMapOverall(ws *websocket.Conn) error {
	m.mapOverallMutex.RLock()
	defer m.mapOverallMutex.RUnlock()
	for _, blkData := range m.MapOverall {
		blk := &wsblk{
			Type: MsgTypeManaMapOverall,
			Data: blkData,
		}
		if err := sendJSON(ws, blk); err != nil {
			return errors.Wrap(err, "failed to send overall mana map to client")
		}
	}
	return nil
}

// StoreMapOnline stores network mana map blk data.
func (m *ManaBuffer) StoreMapOnline(blks ...*ManaNetworkListBlkData) {
	m.mapOnlineMutex.Lock()
	defer m.mapOnlineMutex.Unlock()
	for _, blk := range blks {
		manaType, err := manamodels.TypeFromString(blk.ManaType)
		if err != nil {
			log.Errorf("couldn't parse type of mana: %w", err)
			continue
		}
		m.MapOnline[manaType] = blk
	}
}

// SendMapOnline sends buffered overall mana maps to the provided websocket connection.
func (m *ManaBuffer) SendMapOnline(ws *websocket.Conn) error {
	m.mapOnlineMutex.RLock()
	defer m.mapOnlineMutex.RUnlock()
	for _, blkData := range m.MapOnline {
		blk := &wsblk{
			Type: MsgTypeManaMapOnline,
			Data: blkData,
		}
		if err := sendJSON(ws, blk); err != nil {
			return errors.Wrap(err, "failed to send online mana map to client")
		}
	}
	return nil
}
