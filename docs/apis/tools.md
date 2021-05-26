# Tools API Methods

Tools API allows retrieving technical info about the state of the node.

The API provides the following functions and endpoints:

* [/tools/message/pastcone](#toolsmessagepastcone)
* [/tools/message/missing](#toolsmessagemissing)


Client lib APIs:
* [PastConeExist()](#client-lib---[pastconeexist])
* [Missing()](#client-lib---missing)


##  `/tools/message/pastcone`

Checks that all the messages in the past cone of a message are existing on the node down to the genesis. Returns the number of messages in the past cone as well.

### Parameters

| **Parameter**            | `ID`      |
|--------------------------|----------------|
| **Required or Optional** | required       |
| **Description**          | Message ID  |
| **Type**                 | string         |


### Examples

#### cURL

```shell
curl --location 'http://localhost:8080/tools/message/pastcone?ID=4MSkwAPzGwnjCJmTfbpW4z4GRC7HZHZNS33c2JikKXJc'
```

#### Client lib - `PastConeExist()`

Past cone can be checked using `PastConeExist(base58EncodedMessageID string) (*jsonmodels.PastconeResponse, error)`

```
pastConeCheck, err := goshimAPI.PastConeExist(base58EncodedMessageID)
if err != nil {
    // return error
}

// will print the past cone size
fmt.Println(string(pastConeCheck.PastConeSize))
```

#### Response examples
```json
{
  "exist": true,
  "pastConeSize": 475855
}
```

#### Results

* Returned type

|Return field | Type | Description|
|:-----|:------|:------|
| `exist`  | `bool` | List of known peers. Only returned when parameter is set. |
| `pastConeSize`  | `int` | List of chosen peers. |
| `error` | `string` | Error message. Omitted if success.     |


##  `/tools/message/missing`

Returns all the missing messages and their count.

### Parameters

None

### Examples

#### cURL

```shell
curl --location 'http://localhost:8080/tools/message/missing'
```

#### Client lib - `Missing()`

Missing messages can be retrieved using `Missing() (*jsonmodels.MissingResponse, error)`.

```
missingMsgs, err := goshimAPI.Missing()
if err != nil {
    // return error
}

// will print number of missing messages
fmt.Println(string(missingMsgs.Count))
```

#### Response examples
```json
{
  "ids": [],
  "count": 0
}
```

#### Results

* Returned type

|Return field | Type | Description|
|:-----|:------|:------|
| `ids`  | `[]string` | List of missing messages' IDs. |
| `count`  | `int` | Count of missing messages. |

}