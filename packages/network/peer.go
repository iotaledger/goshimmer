package network

import (
    "io"
    "net"
    "time"
)

type peerImplementation struct {
    timeout             time.Duration
    protocol            string
    conn                net.Conn
    receiveDataHandlers []DataConsumer
    disconnectHandlers  []Callback
    errorHandlers       []ErrorConsumer
}

func NewPeer(protocol string, conn net.Conn) Connection {
    this := &peerImplementation{
        protocol:            protocol,
        conn:                conn,
        receiveDataHandlers: make([]DataConsumer, 0),
        disconnectHandlers:  make([]Callback, 0),
        errorHandlers:       make([]ErrorConsumer, 0),
    }

    return this
}

func (this *peerImplementation) SetTimeout(duration time.Duration) Connection {
    this.timeout = duration

    //this.conn.SetDeadline(time.Now().Add(this.timeout))

    return this
}

func (this *peerImplementation) GetProtocol() string {
    return this.protocol
}

func (this *peerImplementation) GetConnection() net.Conn {
    return this.conn
}

func (this *peerImplementation) Write(data []byte) {
    //this.conn.SetDeadline(time.Now().Add(this.timeout))

    if _, err := this.conn.Write(data); err != nil {
        this.TriggerError(err)
    }
}

func (this *peerImplementation) OnReceiveData(callback DataConsumer) Connection {
    this.receiveDataHandlers = append(this.receiveDataHandlers, callback)

    return this
}

func (this *peerImplementation) OnDisconnect(callback Callback) Connection {
    this.disconnectHandlers = append(this.disconnectHandlers, callback)

    return this
}

func (this *peerImplementation) OnError(callback ErrorConsumer) Connection {
    this.errorHandlers = append(this.errorHandlers, callback)

    return this
}

func (this *peerImplementation) TriggerReceiveData(data []byte) Connection {
    for _, receiveDataHandler := range this.receiveDataHandlers {
        receiveDataHandler(data)
    }

    return this
}

func (this *peerImplementation) TriggerDisconnect() Connection {
    for _, disconnectHandler := range this.disconnectHandlers {
        disconnectHandler()
    }

    return this
}

func (this *peerImplementation) TriggerError(err error) Connection {
    for _, errorHandler := range this.errorHandlers {
        errorHandler(err)
    }

    return this
}

func (this *peerImplementation) HandleConnection() {
    defer this.conn.Close()
    defer this.TriggerDisconnect()

    receiveBuffer := make([]byte, READ_BUFFER_SIZE)
    for {
        //this.conn.SetDeadline(time.Now().Add(this.timeout))

        byteCount, err := this.conn.Read(receiveBuffer)
        if err != nil {
            if err != io.EOF {
                this.TriggerError(err)
            }

            return
        }

        receivedData := make([]byte, byteCount)
        copy(receivedData, receiveBuffer)

        this.TriggerReceiveData(receivedData)
    }
}
