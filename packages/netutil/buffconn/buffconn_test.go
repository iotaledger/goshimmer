package buffconn

import (
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testMsg = []byte("test")

func TestBufferedConnection(t *testing.T) {
	t.Run("Close", func(t *testing.T) {
		conn1, conn2 := net.Pipe()
		buffConn1 := NewBufferedConnection(conn1)
		defer buffConn1.Close()
		buffConn2 := NewBufferedConnection(conn2)
		defer buffConn2.Close()

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			err := buffConn2.Read()
			assert.True(t, errors.Is(err, io.ErrClosedPipe), "unexpected error: %s", err)
			wg.Done()
		}()

		err := buffConn1.Close()
		require.NoError(t, err)
		wg.Wait()
	})

	t.Run("Write", func(t *testing.T) {
		conn1, conn2 := net.Pipe()
		buffConn1 := NewBufferedConnection(conn1)
		defer buffConn1.Close()
		buffConn2 := NewBufferedConnection(conn2)
		defer buffConn2.Close()

		go func() {
			_ = buffConn2.Read()
		}()

		n, err := buffConn1.Write(testMsg)
		require.NoError(t, err)
		assert.EqualValues(t, len(testMsg), n)
	})

	t.Run("ReceiveMessage", func(t *testing.T) {
		conn1, conn2 := net.Pipe()
		buffConn1 := NewBufferedConnection(conn1)
		defer buffConn1.Close()
		buffConn2 := NewBufferedConnection(conn2)
		defer buffConn2.Close()

		var wg sync.WaitGroup
		wg.Add(2)
		buffConn2.Events.ReceiveMessage.Attach(events.NewClosure(func(data []byte) {
			assert.EqualValues(t, testMsg, data)
			wg.Done()
		}))
		go func() {
			err := buffConn2.Read()
			assert.True(t, errors.Is(err, io.EOF), "unexpected error: %s", err)
			wg.Done()
		}()

		n, err := buffConn1.Write(testMsg)
		require.NoError(t, err)
		assert.EqualValues(t, len(testMsg), n)

		time.Sleep(10 * time.Millisecond)

		err = buffConn1.Close()
		require.NoError(t, err)
		wg.Wait()
	})
}
