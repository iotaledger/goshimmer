package buffconn

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/events"
	"go.uber.org/atomic"
)

const (
	// MaxMessageSize is the maximum message size in bytes.
	MaxMessageSize = 4096
	// IOTimeout specifies the timeout for sending and receiving multi packet messages.
	IOTimeout = 4 * time.Second

	headerSize = 4 // size of the header: uint32
)

// Errors returned by the BufferedConnection.
var (
	ErrInvalidHeader      = errors.New("invalid message header")
	ErrInsufficientBuffer = errors.New("insufficient buffer")
)

// BufferedConnectionEvents contains all the events that are triggered during the peer discovery.
type BufferedConnectionEvents struct {
	ReceiveMessage *events.Event
	Close          *events.Event
}

// BufferedConnection is a wrapper for sending and reading messages with a buffer.
type BufferedConnection struct {
	Events BufferedConnectionEvents

	conn                 net.Conn
	incomingHeaderBuffer []byte
	closeOnce            sync.Once

	bytesRead    *atomic.Uint32
	bytesWritten *atomic.Uint32
}

// NewBufferedConnection creates a new BufferedConnection from a net.Conn.
func NewBufferedConnection(conn net.Conn) *BufferedConnection {
	return &BufferedConnection{
		Events: BufferedConnectionEvents{
			ReceiveMessage: events.NewEvent(events.ByteSliceCaller),
			Close:          events.NewEvent(events.CallbackCaller),
		},
		conn:                 conn,
		incomingHeaderBuffer: make([]byte, headerSize),
		bytesRead:            atomic.NewUint32(0),
		bytesWritten:         atomic.NewUint32(0),
	}
}

// Close closes the connection.
// Any blocked Read or Write operations will be unblocked and return errors.
func (c *BufferedConnection) Close() (err error) {
	c.closeOnce.Do(func() {
		err = c.conn.Close()
		// close in separate go routine to avoid deadlocks
		go c.Events.Close.Trigger()
	})
	return err
}

// LocalAddr returns the local network address.
func (c *BufferedConnection) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

// RemoteAddr returns the remote network address.
func (c *BufferedConnection) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

// BytesRead returns the total number of bytes read.
func (c *BufferedConnection) BytesRead() uint32 {
	return c.bytesRead.Load()
}

// BytesWritten returns the total number of bytes written.
func (c *BufferedConnection) BytesWritten() uint32 {
	return c.bytesWritten.Load()
}

// Read starts reading on the connection, it only returns when an error occurred or when Close has been called.
// If a complete message has been received and ReceiveMessage event is triggered with its complete payload.
// If read leads to an error, the loop will be stopped and that error returned.
func (c *BufferedConnection) Read() error {
	buffer := make([]byte, MaxMessageSize)

	for {
		n, err := c.readMessage(buffer)
		if err != nil {
			return err
		}
		if n > 0 {
			c.Events.ReceiveMessage.Trigger(buffer[:n])
		}
	}
}

// Write sends a stream of bytes as messages.
// Each array of bytes you pass in will be pre-pended with it's size. If the
// connection isn't open you will receive an error. If not all bytes can be
// written, Write will keep trying until the full message is delivered, or the
// connection is broken.
func (c *BufferedConnection) Write(msg []byte) (int, error) {
	if l := len(msg); l > MaxMessageSize {
		panic(fmt.Sprintf("invalid message length: %d", l))
	}

	buffer := append(newHeader(len(msg)), msg...)

	if err := c.conn.SetWriteDeadline(time.Now().Add(IOTimeout)); err != nil {
		return 0, fmt.Errorf("error while setting timeout: %w", err)
	}

	toWrite := len(buffer)
	for bytesWritten := 0; bytesWritten < toWrite; {
		n, err := c.conn.Write(buffer[bytesWritten:])
		bytesWritten += n
		c.bytesWritten.Add(uint32(n))
		if err != nil {
			return bytesWritten, err
		}
	}
	return toWrite - headerSize, nil
}

func (c *BufferedConnection) read(buffer []byte) (int, error) {
	toRead := len(buffer)
	for bytesRead := 0; bytesRead < toRead; {
		n, err := c.conn.Read(buffer[bytesRead:])
		bytesRead += n
		c.bytesRead.Add(uint32(n))
		if err != nil {
			return bytesRead, err
		}
	}
	return toRead, nil
}

func (c *BufferedConnection) readMessage(buffer []byte) (int, error) {
	if err := c.conn.SetReadDeadline(time.Time{}); err != nil {
		return 0, fmt.Errorf("error while unsetting timeout: %w", err)
	}
	_, err := c.read(c.incomingHeaderBuffer)
	if err != nil {
		return 0, err
	}

	msgLength, err := parseHeader(c.incomingHeaderBuffer)
	if err != nil {
		return 0, err
	}
	if msgLength > len(buffer) {
		return 0, ErrInsufficientBuffer
	}

	if err := c.conn.SetReadDeadline(time.Now().Add(IOTimeout)); err != nil {
		return 0, fmt.Errorf("error while setting timeout: %w", err)
	}
	return c.read(buffer[:msgLength])
}

func newHeader(msgLength int) []byte {
	// the header only consists of the message length
	header := make([]byte, headerSize)
	binary.BigEndian.PutUint32(header, uint32(msgLength))
	return header
}

func parseHeader(header []byte) (int, error) {
	if len(header) != headerSize {
		return 0, ErrInvalidHeader
	}
	msgLength := int(binary.BigEndian.Uint32(header))
	if msgLength > MaxMessageSize {
		return 0, ErrInvalidHeader
	}
	return msgLength, nil
}
