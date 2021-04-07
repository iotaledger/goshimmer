# Communication Layer APIs

The communication layer represents the base Tangle layer where so called `Messages` are gossiped around. A `Message` contains payloads and it is up to upper layers to interpret and derive functionality out of them.

The API provides three functions to interact with this primitive layer:
* `Data(data []byte) (string, error)`
* `FindMessageByID(base58EncodedIDs []string) (*webapi_message.Response, error)`
* `SendPayload(payload []byte) (string, error)`

#### Issuing a data message
A data message is simply a `Message` containing some raw data (literally bytes). This type of message has therefore no real functionality other than that it is retrievable via `FindMessageByID`.

Example:
```
messageID, err := goshimAPI.Data([]byte("Hello GoShimmer World"))
```

Note that there is no need to do any additional work, since things like tip-selection, PoW and other tasks are done by the node itself.

#### Retrieve messages

Of course messages can then be retrieved via `FindMessageByID()`
```
foundMsgs, err := goshimAPI.FindMessageByID([]string{base58EncodedMessageID})
if err != nil {
    // return error
}

// this might be nil if the message wasn't available
message := foundMsgs[0]
if message == nil {
    // return error
}

// will print "Hello GoShimmer World"
fmt.Println(string(message.Payload))
```

Note that we're getting actual `Message` objects from this call which represent a vertex in the communication layer Tangle. It does not matter what type of payload the message contains, meaning that `FindMessageByID` will also return messages which contain value objects or DRNG payloads.

#### Send Payload
`SendPayload()` takes a `payload` object of any type (data, value, drng, etc.) as a byte slice, issues a message with the given payload and returns its `messageID`. Note, that the payload must be valid, otherwise an error is returned.

Example:
```go
helloPayload := payload.NewData([]byte{"Hello Goshimmer World!"})
messageID, err := goshimAPI.SendPayload(helloPayload.Bytes())
```
