package network

import (
	"io"
	"net"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/events"
)

type ManagedConnection struct {
	Conn         net.Conn
	Events       BufferedConnectionEvents
	readTimeout  time.Duration
	writeTimeout time.Duration
	closeOnce    sync.Once
}

func NewManagedConnection(conn net.Conn) *ManagedConnection {
	bufferedConnection := &ManagedConnection{
		Conn: conn,
		Events: BufferedConnectionEvents{
			ReceiveData: events.NewEvent(dataCaller),
			Close:       events.NewEvent(events.CallbackCaller),
			Error:       events.NewEvent(events.ErrorCaller),
		},
	}

	return bufferedConnection
}

func (this *ManagedConnection) Read(receiveBuffer []byte) (n int, err error) {
	defer this.Close()

	totalReadBytes := 0
	for {
		if err := this.setReadTimeoutBasedDeadline(); err != nil {
			return totalReadBytes, err
		}

		byteCount, err := this.Conn.Read(receiveBuffer)
		if byteCount > 0 {
			totalReadBytes += byteCount

			receivedData := make([]byte, byteCount)
			copy(receivedData, receiveBuffer)

			this.Events.ReceiveData.Trigger(receivedData)
		}

		if err != nil {
			if err != io.EOF {
				this.Events.Error.Trigger(err)
			}

			return totalReadBytes, err
		}
	}
}

func (this *ManagedConnection) Write(data []byte) (n int, err error) {
	if err := this.setWriteTimeoutBasedDeadline(); err != nil {
		return 0, err
	}

	return this.Conn.Write(data)
}

func (this *ManagedConnection) Close() error {
	err := this.Conn.Close()
	if err != nil {
		this.Events.Error.Trigger(err)
	}

	this.closeOnce.Do(func() {
		this.Events.Close.Trigger()
	})

	return err
}

func (this *ManagedConnection) LocalAddr() net.Addr {
	return this.Conn.LocalAddr()
}

func (this *ManagedConnection) RemoteAddr() net.Addr {
	return this.Conn.RemoteAddr()
}

func (this *ManagedConnection) SetDeadline(t time.Time) error {
	return this.Conn.SetDeadline(t)
}

func (this *ManagedConnection) SetReadDeadline(t time.Time) error {
	return this.Conn.SetReadDeadline(t)
}

func (this *ManagedConnection) SetWriteDeadline(t time.Time) error {
	return this.Conn.SetWriteDeadline(t)
}

func (this *ManagedConnection) SetTimeout(d time.Duration) error {
	if err := this.SetReadTimeout(d); err != nil {
		return err
	}

	if err := this.SetWriteTimeout(d); err != nil {
		return err
	}

	return nil
}

func (this *ManagedConnection) SetReadTimeout(d time.Duration) error {
	this.readTimeout = d

	if err := this.setReadTimeoutBasedDeadline(); err != nil {
		return err
	}

	return nil
}

func (this *ManagedConnection) SetWriteTimeout(d time.Duration) error {
	this.writeTimeout = d

	if err := this.setWriteTimeoutBasedDeadline(); err != nil {
		return err
	}

	return nil
}

func (this *ManagedConnection) setReadTimeoutBasedDeadline() error {
	if this.readTimeout != 0 {
		if err := this.Conn.SetReadDeadline(time.Now().Add(this.readTimeout)); err != nil {
			return err
		}
	} else {
		if err := this.Conn.SetReadDeadline(time.Time{}); err != nil {
			return err
		}
	}

	return nil
}

func (this *ManagedConnection) setWriteTimeoutBasedDeadline() error {
	if this.writeTimeout != 0 {
		if err := this.Conn.SetWriteDeadline(time.Now().Add(this.writeTimeout)); err != nil {
			return err
		}
	} else {
		if err := this.Conn.SetWriteDeadline(time.Time{}); err != nil {
			return err
		}
	}

	return nil
}
