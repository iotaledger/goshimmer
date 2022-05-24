---
description: GoShimmer ships with a client Go library which communicates with the HTTP API.
image: /img/logo/goshimmer_light.png
keywords:
- client library
- api
- HTTP API
- golang
---
# Client Lib: Interaction With Layers

:::info

This guide is meant for developers familiar with the Go programming language.

:::

GoShimmer ships with a client Go library which communicates with the HTTP API. Please refer to the [godoc.org docs](https://godoc.org/github.com/iotaledger/goshimmer/client) for function/structure documentation. There is also a set of APIs which do not directly have anything to do with the different layers. Since they are so simple, simply extract their usage from the GoDocs.

# Use the API

Simply `go get` the lib via:
```shell
go get github.com/iotaledger/goshimmer/client
```

Init the API by passing in the API URI of your GoShimmer node:

```go
goshimAPI := client.NewGoShimmerAPI("http://mynode:8080")
```

Optionally, define your own `http.Client` to use, in order for example to define custom timeouts:

```go
goshimAPI := client.NewGoShimmerAPI("http://mynode:8080", client.WithHTTPClient{Timeout: 30 * time.Second})
```

#### A note about errors

The API issues HTTP calls to the defined GoShimmer node. Non 200 HTTP OK status codes will reflect themselves as `error` in the returned arguments. Meaning that for example calling for attachments with a non existing/available transaction on a node, will return an `error` from the respective function. (There might be exceptions to this rule)
