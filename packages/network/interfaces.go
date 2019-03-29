package network

import (
    "net"
    "time"
)

type Connection interface {
    GetProtocol() string
    GetConnection() net.Conn
    Write(data []byte)
    OnReceiveData(callback DataConsumer) Connection
    OnDisconnect(callback Callback) Connection
    OnError(callback ErrorConsumer) Connection
    TriggerReceiveData(data []byte) Connection
    TriggerDisconnect() Connection
    TriggerError(err error) Connection
    SetTimeout(duration time.Duration) Connection
    HandleConnection()
}

type Callback func()

type ErrorConsumer func(err error)

type DataConsumer func(data []byte)

