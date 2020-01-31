---
title: GoShimmer API
language_tabs:
  - go: Go
  - shell: Shell
  - javascript--nodejs: Node.JS
  - python: Python
toc_footers: []
includes: []
search: true
highlight_theme: darkula
headingLevel: 2

---

<h1 id="goshimmer-api">GoShimmer API v0.1.0</h1>

The GoShimmer API provides a simple and consistent way to get transactions from the Tangle, get a node's neighbors, or send new transactions.<br></br>This API accepts HTTP requests and responds with JSON data.

## Base URLs

All requests to this API should be prefixed with the following URL:

```
<a href="http://localhost:8080">http://localhost:8080</a>
```

<h1 id="transactions"></h1>

## POST /broadcastData

Creates a zero-value transaction and attaches it to the Tangle.

Creates a zero-value transaction that includes the given data in the `signatureMessageFragment` field and the given address in the `address` field.<br></br>This endpoint also does tip selection and proof of work before attaching the transaction to the Tangle.

### Body parameters

```json
{
  "address": "string",
  "data": "string"
}
```

<h3 id="post__broadcastdata-parameters">Body parameters</h3>

|**Name**|**Type**|**Required**|**Description**|
|---|---|---|---|
|body|object|true|Request object|
|» address|string|true|Address to add to the transaction's `address` field.|
|» data|string|false|Data to add to the transaction's `signatureMessageFragment` field.<br></br>The data must be no larger than 2187 bytes, and the address must contain only trytes and be either 81 trytes long or 90 trytes long, including a checksum.|

### Examples

--------------------
### Go
```go
package main

import (
       "bytes"
       "net/http"
)

func main() {

    headers := map[string][]string{
        "Content-Type": []string{"application/json"},
        "Accept": []string{"application/json"},
        
    }

    data := bytes.NewBuffer([]byte{jsonReq})
    req, err := http.NewRequest("POST", "http://localhost:8080/broadcastData", data)
    req.Header = headers

    client := &http.Client{}
    resp, err := client.Do(req)
    // ...
}

```
---
### cURL
```bash
# You can also use wget
curl -X POST http://localhost:8080/broadcastData \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json'

```
---
### Node.js
```js
const fetch = require('node-fetch');
const inputBody = '{
  "address": "string",
  "data": "string"
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'

};

fetch('http://localhost:8080/broadcastData',
{
  method: 'POST',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```
---
### Python
```python
import requests
headers = {
  'Content-Type': 'application/json',
  'Accept': 'application/json'
}

r = requests.post('http://localhost:8080/broadcastData', params={

}, headers = headers)

print r.json()

```
--------------------

### Response examples

> 200 Response

```json
{
  "hash": "99IJMBGYVUAYAFAZFGAIVCFWMXP9WTDPX9JDFJLFKNUBLGRRHBERVTTJUZPRRTKKKNMMVX9PYGBKA9999"
}
```

<h3 id="post__broadcastdata-responses">Response examples</h3>

|**Status**|**Meaning**|**Description**|**Schema**|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Successful response|Inline|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|Error response|Inline|

<h3 id="post__broadcastdata-responseschema">Response examples</h3>

Status Code **200**

|**Field**|**Type**|**Description**|
|---|---|---|
|» hash|string|The transaction's hash on the Tangle.|

Status Code **400**

|**Field**|**Type**|**Description**|
|---|---|---|
|» message|string|The error message.|

<aside class="success">
</aside>

## POST /findTransactionHashes

Gets any transaction hashes that were sent to the given addresses.

Searches the Tangle for transactions that contain the given addresses and returns an array of the transactions hashes that were found. The transaction hashes are returned in the same order as the given addresses. For example, if the node doesn't have any transaction hashes for a given address, the value at that index in the returned array is empty.

### Body parameters

```json
{
  "addresses": [
    "string"
  ]
}
```

<h3 id="post__findtransactionhashes-parameters">Body parameters</h3>

|**Name**|**Type**|**Required**|**Description**|
|---|---|---|---|
|body|object|true|Request object|
|» addresses|[string]|true|Addresses to search for in transactions.<br></br>Addresses must contain only trytes and be either 81 trytes long or 90 trytes long, including a checksum.|

### Examples

--------------------
### Go
```go
package main

import (
       "bytes"
       "net/http"
)

func main() {

    headers := map[string][]string{
        "Content-Type": []string{"application/json"},
        "Accept": []string{"application/json"},
        
    }

    data := bytes.NewBuffer([]byte{jsonReq})
    req, err := http.NewRequest("POST", "http://localhost:8080/findTransactionHashes", data)
    req.Header = headers

    client := &http.Client{}
    resp, err := client.Do(req)
    // ...
}

```
---
### cURL
```bash
# You can also use wget
curl -X POST http://localhost:8080/findTransactionHashes \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json'

```
---
### Node.js
```js
const fetch = require('node-fetch');
const inputBody = '{
  "addresses": [
    "string"
  ]
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'

};

fetch('http://localhost:8080/findTransactionHashes',
{
  method: 'POST',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```
---
### Python
```python
import requests
headers = {
  'Content-Type': 'application/json',
  'Accept': 'application/json'
}

r = requests.post('http://localhost:8080/findTransactionHashes', params={

}, headers = headers)

print r.json()

```
--------------------

### Response examples

> 200 Response

```json
{
  "transactions": [
    [
      "string"
    ]
  ]
}
```

<h3 id="post__findtransactionhashes-responses">Response examples</h3>

|**Status**|**Meaning**|**Description**|**Schema**|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Successful response|Inline|

<h3 id="post__findtransactionhashes-responseschema">Response examples</h3>

Status Code **200**

|**Field**|**Type**|**Description**|
|---|---|---|
|» transactions|[array]|

<aside class="success">
</aside>

## POST /getTransactionObjectsByHash

Gets transactions objects for the given transaction hashes

Searches the Tangle for transactions with the given hashes and returns their contents as objects. The transaction objects are returned in the same order as the given hashes. If any of the given hashes is not found, an error is returned.

### Body parameters

```json
{
  "hashes": [
    "string"
  ]
}
```

<h3 id="post__gettransactionobjectsbyhash-parameters">Body parameters</h3>

|**Name**|**Type**|**Required**|**Description**|
|---|---|---|---|
|body|object|true|Request object|
|» hashes|[string]|true|Transaction hashes to search for in the Tangle. <br></br> Transaction hashes must contain only 81 trytes.|

### Examples

--------------------
### Go
```go
package main

import (
       "bytes"
       "net/http"
)

func main() {

    headers := map[string][]string{
        "Content-Type": []string{"application/json"},
        "Accept": []string{"application/json"},
        
    }

    data := bytes.NewBuffer([]byte{jsonReq})
    req, err := http.NewRequest("POST", "http://localhost:8080/getTransactionObjectsByHash", data)
    req.Header = headers

    client := &http.Client{}
    resp, err := client.Do(req)
    // ...
}

```
---
### cURL
```bash
# You can also use wget
curl -X POST http://localhost:8080/getTransactionObjectsByHash \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json'

```
---
### Node.js
```js
const fetch = require('node-fetch');
const inputBody = '{
  "hashes": [
    "string"
  ]
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'

};

fetch('http://localhost:8080/getTransactionObjectsByHash',
{
  method: 'POST',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```
---
### Python
```python
import requests
headers = {
  'Content-Type': 'application/json',
  'Accept': 'application/json'
}

r = requests.post('http://localhost:8080/getTransactionObjectsByHash', params={

}, headers = headers)

print r.json()

```
--------------------

### Response examples

> 200 Response

```json
{
  "transaction": [
    {
      "hash": "string",
      "weightMagnitude": 0,
      "trunkTransactionHash": "string",
      "branchTransactionHash": "string",
      "head": true,
      "tail": true,
      "nonce": "string",
      "address": "string",
      "timestamp": 0,
      "signatureMessageFragment": "string"
    }
  ]
}
```

<h3 id="post__gettransactionobjectsbyhash-responses">Response examples</h3>

|**Status**|**Meaning**|**Description**|**Schema**|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Successful response|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Transaction(s) not found|None|

<h3 id="post__gettransactionobjectsbyhash-responseschema">Response examples</h3>

Status Code **200**

|**Field**|**Type**|**Description**|
|---|---|---|
|» transaction|[[Transaction](#schematransaction)]|
|»» hash|string|
|»» weightMagnitude|integer|
|»» trunkTransactionHash|string|
|»» branchTransactionHash|string|
|»» head|boolean|
|»» tail|boolean|
|»» nonce|string|
|»» address|string|
|»» timestamp|integer|
|»» signatureMessageFragment|string|

<aside class="success">
</aside>

## POST /getTransactionTrytesByHash

Gets the transaction trytes of given transaction hashes.

Searches the Tangle for transactions with the given hashes and returns their contents in trytes. The transaction trytes are returned in the same order as the given hashes. If any of the given hashes is not found, an error is returned.

### Body parameters

```json
{
  "hashes": [
    "string"
  ]
}
```

<h3 id="post__gettransactiontrytesbyhash-parameters">Body parameters</h3>

|**Name**|**Type**|**Required**|**Description**|
|---|---|---|---|
|body|object|true|Request object|
|» hashes|[string]|true|Transaction hashes to search for in the Tangle. <br></br> Transaction hashes must contain only 81 trytes.|

### Examples

--------------------
### Go
```go
package main

import (
       "bytes"
       "net/http"
)

func main() {

    headers := map[string][]string{
        "Content-Type": []string{"application/json"},
        "Accept": []string{"application/json"},
        
    }

    data := bytes.NewBuffer([]byte{jsonReq})
    req, err := http.NewRequest("POST", "http://localhost:8080/getTransactionTrytesByHash", data)
    req.Header = headers

    client := &http.Client{}
    resp, err := client.Do(req)
    // ...
}

```
---
### cURL
```bash
# You can also use wget
curl -X POST http://localhost:8080/getTransactionTrytesByHash \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json'

```
---
### Node.js
```js
const fetch = require('node-fetch');
const inputBody = '{
  "hashes": [
    "string"
  ]
}';
const headers = {
  'Content-Type':'application/json',
  'Accept':'application/json'

};

fetch('http://localhost:8080/getTransactionTrytesByHash',
{
  method: 'POST',
  body: inputBody,
  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```
---
### Python
```python
import requests
headers = {
  'Content-Type': 'application/json',
  'Accept': 'application/json'
}

r = requests.post('http://localhost:8080/getTransactionTrytesByHash', params={

}, headers = headers)

print r.json()

```
--------------------

### Response examples

> 200 Response

```json
{
  "trytes": [
    "string"
  ]
}
```

<h3 id="post__gettransactiontrytesbyhash-responses">Response examples</h3>

|**Status**|**Meaning**|**Description**|**Schema**|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Successful response|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|Transactions not found|None|

<h3 id="post__gettransactiontrytesbyhash-responseschema">Response examples</h3>

Status Code **200**

|**Field**|**Type**|**Description**|
|---|---|---|
|» trytes|[string]|

<aside class="success">
</aside>

## GET /getTransactionsToApprove

Gets two tip transactions from the Tangle.

Runs the tip selection algorithm and returns two tip transactions hashes. <br></br>You can use these hashes in the branch and trunk transaction fields of a new transaction.

### Examples

--------------------
### Go
```go
package main

import (
       "bytes"
       "net/http"
)

func main() {

    headers := map[string][]string{
        "Accept": []string{"application/json"},
        
    }

    data := bytes.NewBuffer([]byte{jsonReq})
    req, err := http.NewRequest("GET", "http://localhost:8080/getTransactionsToApprove", data)
    req.Header = headers

    client := &http.Client{}
    resp, err := client.Do(req)
    // ...
}

```
---
### cURL
```bash
# You can also use wget
curl -X GET http://localhost:8080/getTransactionsToApprove \
  -H 'Accept: application/json'

```
---
### Node.js
```js
const fetch = require('node-fetch');

const headers = {
  'Accept':'application/json'

};

fetch('http://localhost:8080/getTransactionsToApprove',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```
---
### Python
```python
import requests
headers = {
  'Accept': 'application/json'
}

r = requests.get('http://localhost:8080/getTransactionsToApprove', params={

}, headers = headers)

print r.json()

```
--------------------

### Response examples

> 200 Response

```json
{
  "branchTransaction": "string",
  "trunkTransaction": "string"
}
```

<h3 id="get__gettransactionstoapprove-responses">Response examples</h3>

|**Status**|**Meaning**|**Description**|**Schema**|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Successful response|Inline|

<h3 id="get__gettransactionstoapprove-responseschema">Response examples</h3>

Status Code **200**

|**Field**|**Type**|**Description**|
|---|---|---|
|» branchTransaction|string|
|» trunkTransaction|string|

<aside class="success">
</aside>

## GET /spammer

Sends spam transactions.

Sends zero-value transactions at the given rate per second.<br></br>You can start the spammer, using the `cmd=start` command and stop it, using the `cmd=stop` command. Optionally, a parameter `tps` can be provided (i.e., `tps=10`) to change the default rate (`tps=1`).

<h3 id="get__spammer-parameters">Body parameters</h3>

|**Name**|**Type**|**Required**|**Description**|
|---|---|---|---|
|cmd|string|true|Command to either `start` or `stop` spamming.|
|tps|integer|false|Change the sending rate.|

#### Enumerated Values

|Parameter|Value|
|---|---|
|cmd|start|
|cmd|stop|

### Examples

--------------------
### Go
```go
package main

import (
       "bytes"
       "net/http"
)

func main() {

    data := bytes.NewBuffer([]byte{jsonReq})
    req, err := http.NewRequest("GET", "http://localhost:8080/spammer", data)
    req.Header = headers

    client := &http.Client{}
    resp, err := client.Do(req)
    // ...
}

```
---
### cURL
```bash
# You can also use wget
curl -X GET http://localhost:8080/spammer?cmd=start

```
---
### Node.js
```js
const fetch = require('node-fetch');

fetch('http://localhost:8080/spammer?cmd=start',
{
  method: 'GET'

})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```
---
### Python
```python
import requests

r = requests.get('http://localhost:8080/spammer', params={
  'cmd': 'start'
)

print r.json()

```
--------------------

<h3 id="get__spammer-responses">Response examples</h3>

|**Status**|**Meaning**|**Description**|**Schema**|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Successful Response|None|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|invalid command in request|None|

<aside class="success">
</aside>

<h1 id="neighbors"></h1>

## GET /getNeighbors

Gets the node's chosen and accepted neighbors.

Returns the node's chosen and accepted neighbors. Optionally, you can pass the `known=1` query parameter to return all known peers.

<h3 id="get__getneighbors-parameters">Body parameters</h3>

|**Name**|**Type**|**Required**|**Description**|
|---|---|---|---|
|known|integer|false|Returns all known peers when set to 1.|

### Examples

--------------------
### Go
```go
package main

import (
       "bytes"
       "net/http"
)

func main() {

    headers := map[string][]string{
        "Accept": []string{"application/json"},
        
    }

    data := bytes.NewBuffer([]byte{jsonReq})
    req, err := http.NewRequest("GET", "http://localhost:8080/getNeighbors", data)
    req.Header = headers

    client := &http.Client{}
    resp, err := client.Do(req)
    // ...
}

```
---
### cURL
```bash
# You can also use wget
curl -X GET http://localhost:8080/getNeighbors \
  -H 'Accept: application/json'

```
---
### Node.js
```js
const fetch = require('node-fetch');

const headers = {
  'Accept':'application/json'

};

fetch('http://localhost:8080/getNeighbors',
{
  method: 'GET',

  headers: headers
})
.then(function(res) {
    return res.json();
}).then(function(body) {
    console.log(body);
});

```
---
### Python
```python
import requests
headers = {
  'Accept': 'application/json'
}

r = requests.get('http://localhost:8080/getNeighbors', params={

}, headers = headers)

print r.json()

```
--------------------

### Response examples

> 200 Response

```json
{
  "chosen": [
    {
      "id": "V8LYtWWcPYYDTTXLeIEFjJEuWlsjDiI0+Pq",
      "publicKey": "V8LYtWWcPYYDTTXLeIEFjJEuWlsjDiI0+Pq",
      "services": [
        {
          "id": "peering",
          "address": "198.51.100.1:80"
        }
      ]
    }
  ],
  "accepted": [
    {
      "id": "V8LYtWWcPYYDTTXLeIEFjJEuWlsjDiI0+Pq",
      "publicKey": "V8LYtWWcPYYDTTXLeIEFjJEuWlsjDiI0+Pq",
      "services": [
        {
          "id": "peering",
          "address": "198.51.100.1:80"
        }
      ]
    }
  ],
  "known": [
    {
      "id": "V8LYtWWcPYYDTTXLeIEFjJEuWlsjDiI0+Pq",
      "publicKey": "V8LYtWWcPYYDTTXLeIEFjJEuWlsjDiI0+Pq",
      "services": [
        {
          "id": "peering",
          "address": "198.51.100.1:80"
        }
      ]
    }
  ]
}
```

<h3 id="get__getneighbors-responses">Response examples</h3>

|**Status**|**Meaning**|**Description**|**Schema**|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|Successful response|Inline|
|501|[Not Implemented](https://tools.ietf.org/html/rfc7231#section-6.6.2)|Neighbor Selection/Discovery is not enabled|None|

<h3 id="get__getneighbors-responseschema">Response examples</h3>

Status Code **200**

|**Field**|**Type**|**Description**|
|---|---|---|
|» chosen|[[Peer](#schemapeer)]|
|»» id|string|
|»» publicKey|string|
|»» services|[[PeerService](#schemapeerservice)]|
|»»» id|string|
|»»» address|string|
|»» accepted|[[Peer](#schemapeer)]|
|»» known|[[Peer](#schemapeer)]|

<aside class="success">
</aside>

# Schemas

<h2 id="tocSpeer">Peer</h2>

<a id="schemapeer"></a>

```json
{
  "id": "V8LYtWWcPYYDTTXLeIEFjJEuWlsjDiI0+Pq",
  "publicKey": "V8LYtWWcPYYDTTXLeIEFjJEuWlsjDiI0+Pq",
  "services": [
    {
      "id": "peering",
      "address": "198.51.100.1:80"
    }
  ]
}

```

### Properties

|**Name**|**Type**|**Required**|**Description**|
|---|---|---|---|
|id|string|false|ID of the peer node.|
|publicKey|string|false|Public key of the peer node.|
|services|[[PeerService](#schemapeerservice)]|false|Services that the peer node is running.|

<h2 id="tocSpeerservice">PeerService</h2>

<a id="schemapeerservice"></a>

```json
{
  "id": "peering",
  "address": "198.51.100.1:80"
}

```

### Properties

|**Name**|**Type**|**Required**|**Description**|
|---|---|---|---|
|id|string|false|ID of the service. Can be "peering", "gossip", or "fpc".|
|address|string|false|The IP address and port that the service is using.|

<h2 id="tocStransaction">Transaction</h2>

<a id="schematransaction"></a>

```json
{
  "hash": "string",
  "weightMagnitude": 0,
  "trunkTransactionHash": "string",
  "branchTransactionHash": "string",
  "head": true,
  "tail": true,
  "nonce": "string",
  "address": "string",
  "timestamp": 0,
  "signatureMessageFragment": "string"
}

```

### Properties

|**Name**|**Type**|**Required**|**Description**|
|---|---|---|---|
|hash|string|false|Transaction hash.|
|weightMagnitude|integer|false|The weight magnitude of the transaction hash.|
|trunkTransactionHash|string|false|The transaction's trunk transaction hash.|
|branchTransactionHash|string|false|The transaction's branch transaction hash.|
|head|boolean|false|Whether this transaction is the head transaction in its bundle.|
|tail|boolean|false|Whether this transaction is the tail transaction in its bundle.|
|nonce|string|false|The transaction's nonce, which is used to validate the proof of work.|
|address|string|false|The address of the transaction.|
|timestamp|integer|false|The Unix epoch at which the transaction was created.|
|signatureMessageFragment|string|false|The transaction's signature or message.|

