package dashboard

import (
	"sync"

	"github.com/gorilla/websocket"
	"golang.org/x/xerrors"

	"github.com/iotaledger/goshimmer/packages/mana"
)

const (
	maxManaEventsBufferSize = 100
	maxManaValuesBufferSize = 100
)

// ManaBuffer holds recent data related to mana in the dashboard. Used to fill frontend on page load/reload.
type ManaBuffer struct {
	// Events store PledgedEvent and RevokedEvent structs in chronological order.
	Events          []mana.Event
	eventsMutex     sync.RWMutex
	ValueMsgs       []*manaValueMsgData
	valueMsgsMutex  sync.RWMutex
	MapOverall      map[mana.Type]*manaNetworkListMsgData
	mapOverallMutex sync.RWMutex
	MapOnline       map[mana.Type]*manaNetworkListMsgData
	mapOnlineMutex  sync.RWMutex
}

// NewManaBuffer creates and initializes a new, empty buffer.
func NewManaBuffer() *ManaBuffer {
	return &ManaBuffer{
		Events:     make([]mana.Event, 0),
		ValueMsgs:  make([]*manaValueMsgData, 0),
		MapOverall: make(map[mana.Type]*manaNetworkListMsgData),
		MapOnline:  make(map[mana.Type]*manaNetworkListMsgData),
	}
}

// StoreEvent stores an event in the buffer. If it is full, drops the oldest event.
func (m *ManaBuffer) StoreEvent(event mana.Event) {
	m.eventsMutex.Lock()
	defer m.eventsMutex.Unlock()
	if len(m.Events) >= maxManaEventsBufferSize {
		// drop oldest event if buffer is full
		m.Events = m.Events[1:]
	}
	m.Events = append(m.Events, event)
}

// SendEvents send all events in the buffer through the provided websocket connection.
func (m *ManaBuffer) SendEvents(ws *websocket.Conn) error {
	m.eventsMutex.RLock()
	defer m.eventsMutex.RUnlock()
	for _, ev := range m.Events {
		var msg *wsmsg
		switch ev.Type() {
		case mana.EventTypePledge:
			msg = &wsmsg{
				Type: MsgTypeManaPledge,
				Data: ev.ToJSONSerializable(),
			}
		case mana.EventTypeRevoke:
			msg = &wsmsg{
				Type: MsgTypeManaRevoke,
				Data: ev.ToJSONSerializable(),
			}
		default:
			return xerrors.Errorf("unexpected mana event type")
		}
		if err := sendJSON(ws, msg); err != nil {
			return xerrors.Errorf("failed to send mana event to client: %w", err)
		}
	}
	return nil
}

// StoreValueMsg stores a value msg in the buffer. If it is full, drops the oldest msg.
func (m *ManaBuffer) StoreValueMsg(msg *manaValueMsgData) {
	m.valueMsgsMutex.Lock()
	defer m.valueMsgsMutex.Unlock()
	if len(m.ValueMsgs) >= maxManaValuesBufferSize {
		// drop oldest msg if buffer is full
		m.ValueMsgs = m.ValueMsgs[1:]
	}
	m.ValueMsgs = append(m.ValueMsgs, msg)
}

// SendValueMsgs sends all msgs in the buffer through the provided websocket connection.
func (m *ManaBuffer) SendValueMsgs(ws *websocket.Conn) error {
	m.valueMsgsMutex.RLock()
	defer m.valueMsgsMutex.RUnlock()
	for _, valueMsg := range m.ValueMsgs {
		var msg = &wsmsg{
			Type: MsgTypeManaValue,
			Data: valueMsg,
		}
		if err := sendJSON(ws, msg); err != nil {
			return xerrors.Errorf("failed to send mana value to client: %w", err)
		}
	}
	return nil
}

// StoreMapOverall stores network mana map msg data.
func (m *ManaBuffer) StoreMapOverall(msgs ...*manaNetworkListMsgData) {
	m.mapOverallMutex.Lock()
	defer m.mapOverallMutex.Unlock()
	for _, msg := range msgs {
		manaType, err := mana.TypeFromString(msg.ManaType)
		if err != nil {
			log.Errorf("couldn't parse type of mana: %w", err)
			continue
		}
		m.MapOverall[manaType] = msg
	}
}

// SendMapOverall sends buffered overall mana maps to the provided websocket connection.
func (m *ManaBuffer) SendMapOverall(ws *websocket.Conn) error {
	m.mapOverallMutex.RLock()
	defer m.mapOverallMutex.RUnlock()
	for _, msgData := range m.MapOverall {
		msg := &wsmsg{
			Type: MsgTypeManaMapOverall,
			Data: msgData,
		}
		if err := sendJSON(ws, msg); err != nil {
			return xerrors.Errorf("failed to send overall mana map to client: %w", err)
		}
	}
	return nil
}

// StoreMapOnline stores network mana map msg data.
func (m *ManaBuffer) StoreMapOnline(msgs ...*manaNetworkListMsgData) {
	m.mapOnlineMutex.Lock()
	defer m.mapOnlineMutex.Unlock()
	for _, msg := range msgs {
		manaType, err := mana.TypeFromString(msg.ManaType)
		if err != nil {
			log.Errorf("couldn't parse type of mana: %w", err)
			continue
		}
		m.MapOnline[manaType] = msg
	}
}

// SendMapOnline sends buffered overall mana maps to the provided websocket connection.
func (m *ManaBuffer) SendMapOnline(ws *websocket.Conn) error {
	m.mapOnlineMutex.RLock()
	defer m.mapOnlineMutex.RUnlock()
	for _, msgData := range m.MapOnline {
		msg := &wsmsg{
			Type: MsgTypeManaMapOnline,
			Data: msgData,
		}
		if err := sendJSON(ws, msg); err != nil {
			return xerrors.Errorf("failed to send online mana map to client: %w", err)
		}
	}
	return nil
}
