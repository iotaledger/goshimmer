---
description: The Spammer tool lets you add messages to the tangle when running GoShimmer.
image: /img/logo/goshimmer_light.png
keywords:
- client library
- HTTP API
- spammer
- add messages
- interval
- tangle
---
# Spammer API Methods

The Spammer tool lets you add messages to the tangle when running GoShimmer.
**Note:** Make sure you enable the **spammer plugin** before interacting with the API.

The API provides the following functions and endpoints:

* [/spammer](#spammer)


Client lib APIs:
* [ToggleSpammer()](#client-lib---togglespammer)

##  `/spammer`

In order to start the spammer, you need to send GET requests to a `/spammer` API endpoint with the following parameters:

### Parameters

| **Parameter**            | `cmd`      |
|--------------------------|----------------|
| **Required or Optional** | required       |
| **Description**          | Action to perform. One of two possible values: `start` and `stop`.   |
| **Type**                 | `string`         |



| **Parameter**            | `rate`      |
|--------------------------|----------------|
| **Required or Optional** | optional       |
| **Description**          | Messages per time unit. Only applicable when `cmd=start`. (default: 1)  |
| **Type**                 | `int`         |


| **Parameter**            | `unit`      |
|--------------------------|----------------|
| **Required or Optional** | optional       |
| **Description**          | Indicates the unit for the spam rate: message per minute or second. One of two possible values: `mpm` and `mps`. (default: `mps`) |
| **Type**                 | `string`         |


| **Parameter**            | `imif` (Inter Message Issuing Function)     |
|--------------------------|----------------|
| **Required or Optional** | optional       |
| **Description**          | Parameter indicating time interval between issued messages. Possible values: `poisson`, `uniform`. |
| **Type**                 | `string`         |


Description of `imif` values:
* `poisson` - emit messages modeled with Poisson point process, whose time intervals are exponential variables with mean 1/rate
* `uniform` - issues messages at constant rate 

### Examples

#### cURL

```shell
curl --location 'http://localhost:8080/spammer?cmd=start&rate=100'
curl --location 'http://localhost:8080/spammer?cmd=start&rate=100&imif=uniform&unit=mpm'
curl --location 'http://localhost:8080/spammer?cmd=stop'
```

#### Client lib - `ToggleSpammer()`

Spammer can be enabled and disabled via `ToggleSpammer(enable bool, rate int, imif string) (*jsonmodels.SpammerResponse, error)`
```go
res, err := goshimAPI.ToggleSpammer(true, 100, "mps", "uniform")
if err != nil {
    // return error
}

// will print the response
fmt.Println(res.Message)
```

#### Response examples

```json
{"message": "started spamming messages"}
```

#### Results

|Return field | Type | Description|
|:-----|:------|:------|
| `message`  | `string` | Message with resulting message. |
| `error` | `string` | Error message. Omitted if success.     |
