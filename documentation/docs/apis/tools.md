---
description: Tools API allows retrieving technical info about the state of the node.
image: /img/logo/goshimmer_light.png
keywords:
- client library
- HTTP API
- tools
- message
- past cone
- diagnostic
---
# Tools API Methods

Tools API allows retrieving technical info about the state of the node.

The API provides the following functions and endpoints:

* [/tools/message/pastcone](#toolsmessagepastcone)
* [/tools/message/missing](#toolsmessagemissing)
* [/tools/message/approval](#tools/message/approval)
* [/tools/message/orphanage](#toolsmessageorphanage)
* [tools/diagnostic/messages](#toolsdiagnosticmessages)
* [tools/diagnostic/messages/firstweakreferences](#toolsdiagnosticmessagesfirstweakreferences)
* [tools/diagnostic/messages/rank/:rank](#toolsdiagnosticmessagesrankrank)
* [tools/diagnostic/utxodag](#toolsdiagnosticutxodag)
* [tools/diagnostic/branches](#toolsdiagnosticbranches)
* [tools/diagnostic/branches/lazybooked](#toolsdiagnosticbrancheslazybooked)
* [tools/diagnostic/branches/invalid](#toolsdiagnosticbranchesinvalid)
* [tools/diagnostic/tips](#toolsdiagnostictips)
* [tools/diagnostic/tips/strong](#toolsdiagnostictipsstrong)
* [tools/diagnostic/tips/weak](#toolsdiagnostictipsweak)
* [tools/diagnostic/drng](#toolsdiagnosticdrng)


Client lib APIs:
* [PastConeExist()](#client-lib---pastconeexist)
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

```go
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

|Return field | Type | Description|
|:-----|:------|:------|
| `exist`  | `bool` | A boolean indicates if the message and its past cone exist. |
| `pastConeSize`  | `int` | Size of the past cone of the given message. |
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

```go
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

|Return field | Type | Description|
|:-----|:------|:------|
| `ids`  | `[]string` | List of missing messages' IDs. |
| `count`  | `int` | Count of missing messages. |


##  `/tools/message/approval`

Returns the first approver of all messages.

### Parameters

None

### Examples

#### cURL

```shell
curl --location 'http://localhost:8080/tools/message/approval'
```

#### Response examples
The response is written in a csv file.

## `tools/message/orphanage`

Returns orphaned messages of the future cone of the given message ID.

### Parameters

| **Parameter**            | `msgID`      |
|--------------------------|----------------|
| **Required or Optional** | required       |
| **Description**          | Message ID encoded in Bases58 |
| **Type**                 | string         |


### Examples

#### cURL

```shell
curl --location 'http://localhost:8080/tools/message/orphanage?msgID=4MSkwAPzGwnjCJmTfbpW4z4GRC7HZHZNS33c2JikKXJc'
```

#### Response examples
The response is written in a csv file.
```csv
MsgID,MsgIssuerID,MsgIssuanceTime,MsgArrivalTime,MsgSolidTime,MsgApprovedBy
...

7h7arHrxYhuuzgpvRtuw6jn5AwtAA5AEiKnAzdQheyDW,dAnF7pQ6k7a,1622100376301474621,1622100390350323240,1622100390350376317,true
```

## `tools/diagnostic/messages`
Returns all the messages in the storage.

### Parameters

None

### Examples

#### cURL

```shell
curl --location 'http://localhost:8080/tools/diagnostic/messages'
```

#### Response examples
The response is written in a csv file.
```csv
ID,IssuerID,IssuerPublicKey,IssuanceTime,ArrivalTime,SolidTime,ScheduledTime,BookedTime,GradeOfFinality,GradeOfFinalityTime,StrongParents,WeakParents,DislikeParents,LikeParents,StrongApprovers,WeakApprovers,BranchID,Scheduled,Booked,Invalid,Rank,IsPastMarker,PastMarkers,PMHI,PMLI,FutureMarkers,FMHI,FMLI,PayloadType,TransactionID

...

7h7arHrxYhuuzgpvRtuw6jn5AwtAA5AEiKnAzdQheyDW,dAnF7pQ6k7a,CHfU1NUf6ZvUKDQHTG2df53GR7CvuMFtyt7YymJ6DwS3,1622100376301474621,1622100390350323240,1622100390350376317,1622100390350655597,1622100390497058485,1622100394498368012,GradeOfFinanlity(3),E8jiyKgouhbk8GK8xNiwSnLM4FSzmCfvCmBijbKd8z8A,,,E8jiyKgouhbk8GK8xNiwSnLM4FSzmCfvCmBijbKd8z8A,BranchID(MasterBranchID),true,true,true,1,0:0,0,0,1:2,2,2,TransactionType(1337)
```

## `tools/diagnostic/messages/firstweakreferences`

Returns the first weak reference of all messages in the storage.

### Parameters

None

### Examples

#### cURL

```shell
curl --location 'http://localhost:8080/tools/diagnostic/messages/firstweakreferences'
```

#### Response examples
The response is written in a csv file.
```
ID,IssuerID,IssuerPublicKey,IssuanceTime,ArrivalTime,SolidTime,ScheduledTime,BookedTime,GradeOfFinality,GradeOfFinalityTime,StrongParents,WeakParents,DislikeParents,LikeParents,StrongApprovers,WeakApprovers,BranchID,Scheduled,Booked,Invalid,Rank,IsPastMarker,PastMarkers,PMHI,PMLI,FutureMarkers,FMHI,FMLI,PayloadType,TransactionID

...

7h7arHrxYhuuzgpvRtuw6jn5AwtAA5AEiKnAzdQheyDW,dAnF7pQ6k7a,CHfU1NUf6ZvUKDQHTG2df53GR7CvuMFtyt7YymJ6DwS3,1622100376301474621,1622100390350323240,1622100390350376317,1622100390350655597,1622100390497058485,1622100394498368012,GradeOfFinanlity(3),E8jiyKgouhbk8GK8xNiwSnLM4FSzmCfvCmBijbKd8z8A,,,E8jiyKgouhbk8GK8xNiwSnLM4FSzmCfvCmBijbKd8z8A,BranchID(MasterBranchID),true,true,true,1,0:0,0,0,1:2,2,2,TransactionType(1337)
```

## `tools/diagnostic/messages/rank/:rank`
Returns a list of messages with rank >= of the given rank parameter.
### Parameters

| **Parameter**            | `rank`      |
|--------------------------|----------------|
| **Required or Optional**   | Required     |
| **Description**   | message rank      |
| **Type**      | uint64      |

### Examples

#### cURL

```shell
curl --location 'http://localhost:8080/tools/diagnostic/messages/rank/:rank'
```
where `:rank` is the uint64, e.g. 20.

#### Response examples
The response is written in a csv file.
```
ID,IssuerID,IssuerPublicKey,IssuanceTime,ArrivalTime,SolidTime,ScheduledTime,BookedTime,GradeOfFinality,GradeOfFinalityTime,StrongParents,WeakParents,DislikeParents,LikeParents,StrongApprovers,WeakApprovers,BranchID,Scheduled,Booked,Invalid,Rank,IsPastMarker,PastMarkers,PMHI,PMLI,FutureMarkers,FMHI,FMLI,PayloadType,TransactionID

...

7h7arHrxYhuuzgpvRtuw6jn5AwtAA5AEiKnAzdQheyDW,dAnF7pQ6k7a,CHfU1NUf6ZvUKDQHTG2df53GR7CvuMFtyt7YymJ6DwS3,1622100376301474621,1622100390350323240,1622100390350376317,1622100390350655597,1622100390497058485,1622100394498368012,GradeOfFinanlity(3),E8jiyKgouhbk8GK8xNiwSnLM4FSzmCfvCmBijbKd8z8A,,,E8jiyKgouhbk8GK8xNiwSnLM4FSzmCfvCmBijbKd8z8A,BranchID(MasterBranchID),true,true,true,1,0:0,0,0,1:2,2,2,TransactionType(1337)
```

## `tools/diagnostic/utxodag`
Returns the information of all transactions in the storage.
### Parameters

None.

### Examples

#### cURL

```shell
curl --location 'http://localhost:8080/tools/diagnostic/utxodag'
```

#### Response examples
The response is written in a csv file.
```
ID,IssuanceTime,SolidTime,AccessManaPledgeID,ConsensusManaPledgeID,Inputs,Outputs,Attachments,BranchID,Conflicting,LazyBooked,GradeOfFinality,GradeOfFinalityTime

...

uNUZMoAdYZu74ZREoZr84AbYb9du1fC8vTbXpsX3rj6,1622102040372947362,1622102040419353230,1622102044420491940,2GtxMQD94KvDH1SJPJV7icxofkyV1njuUZKtsqKmtux5,2GtxMQD94KvDH1SJPJV7icxofkyV1njuUZKtsqKmtux5,DBejuv32xNJdZQurbitPTktm5HJML5SdnmN6ic6xQGKd:83,uNUZMoAdYZu74ZREoZr84AbYb9du1fC8vTbXpsX3rj6:0,3Lu696zF21tCAeqX7mEjwC1xPocWMnQVHAPMtd9CCdep,BranchID(MasterBranchID),true,true,GradeOfFinality(3),1622102040419353230
```

## `tools/diagnostic/branches`
Returns the information of all conflict and aggregated branches in the storag.

### Parameters

None.

### Examples

#### cURL

```shell
curl --location 'http://localhost:8080/tools/diagnostic/branches'
```

#### Response examples
The response is written in a csv file.
```
ID,ConflictSet,IssuanceTime,SolidTime,LazyBooked,GradeOfFinality

...

7h7arHrxYhuuzgpvRtuw6jn5AwtAA5AEiKnAzdQheyDW,CHfU1NUf6ZvUKDQHTG2df53GR7CvuMFtyt7YymJ6DwS3,1622100376301474621,1622100390350323240,true,3
```


## `tools/diagnostic/branches/lazybooked`
Returns the information of all lazy booked branches.
### Parameters

None.

### Examples

#### cURL

```shell
curl --location 'http://localhost:8080/tools/diagnostic/branches/lazybooked'
```

#### Response examples
The response is written in a csv file.
```
ID,ConflictSet,IssuanceTime,SolidTime,LazyBooked,GradeOfFinality

...

7h7arHrxYhuuzgpvRtuw6jn5AwtAA5AEiKnAzdQheyDW,CHfU1NUf6ZvUKDQHTG2df53GR7CvuMFtyt7YymJ6DwS3,1622100376301474621,1622100390350323240,true,3
```

## `tools/diagnostic/branches/invalid`
Returns the information of all invalid branches.
### Parameters

None.

### Examples

#### cURL

```shell
curl --location 'http://localhost:8080/tools/diagnostic/branches/invalid'
```

#### Response examples
The response is written in a csv file.
```
ID,ConflictSet,IssuanceTime,SolidTime,LazyBooked,GradeOfFinality

...

7h7arHrxYhuuzgpvRtuw6jn5AwtAA5AEiKnAzdQheyDW,CHfU1NUf6ZvUKDQHTG2df53GR7CvuMFtyt7YymJ6DwS3,1622100376301474621,1622100390350323240,true,3
```

## `tools/diagnostic/tips`
Returns the information of all tips.
### Parameters

None.

### Examples

#### cURL

```shell
curl --location 'http://localhost:8080/tools/diagnostic/tips'
```

#### Response examples
The response is written in a csv file.
```
tipType,ID,IssuerID,IssuerPublicKey,IssuanceTime,ArrivalTime,SolidTime,ScheduledTime,BookedTime,GradeOfFinality,GradeOfFinalityTime,StrongParents,WeakParents,DislikeParents,LikeParents,StrongApprovers,WeakApprovers,BranchID,Scheduled,Booked,Invalid,Rank,IsPastMarker,PastMarkers,PMHI,PMLI,FutureMarkers,FMHI,FMLI,PayloadType,TransactionID

...

TipType(StrongTip),7h7arHrxYhuuzgpvRtuw6jn5AwtAA5AEiKnAzdQheyDW,dAnF7pQ6k7a,CHfU1NUf6ZvUKDQHTG2df53GR7CvuMFtyt7YymJ6DwS3,1622100376301474621,1622100390350323240,1622100390350376317,1622100390350655597,1622100390497058485,1622100394498368012,GradeOfFinanlity(3),E8jiyKgouhbk8GK8xNiwSnLM4FSzmCfvCmBijbKd8z8A,,,E8jiyKgouhbk8GK8xNiwSnLM4FSzmCfvCmBijbKd8z8A,BranchID(MasterBranchID),true,true,true,1,0:0,0,0,1:2,2,2,TransactionType(1337)
```

## `tools/diagnostic/drng`
Returns the information of all dRNG messages.
### Parameters

None.

### Examples

#### cURL

```shell
curl --location 'http://localhost:8080/tools/diagnostic/drng'
```

#### Response examples
The response is written in a csv file.
```
ID,IssuerID,IssuerPublicKey,IssuanceTime,ArrivalTime,SolidTime,ScheduledTime,BookedTime,dRNGPayloadType,InstanceID,Round,PreviousSignature,Signature,DistributedPK

...

BsSw31y4BufNoPp93TRfgDfXdrjnevsm7Up2mHtybzdK,CRPFWYijV1T,GUdTwLDb6t6vZ7X5XzEnjFNDEVPteU7tVQ9nzKLfPjdo,1621963390710701221,1621963391011749004,1621963391011818075,1621963391011903917,1621963391012012853,dRNG(111),1339,2210960,us8vrWKdKtNvXdx424hgqGYpM65Cs2KAGmAyhinCncn6PQ8Dv4hLh1rZ3ugvk2QZkGofJhwNvx2EmD5Vzcz3RQTowfiNBTpLJYEUM4swAPXaFwSGntWhvWDYtpyHrXtGtBP,24LuByAUakW36DmEyCz58Ld5utTeKh3zCUbJ4mn6Eo6rZmhb7wnZnjQN3KMm59TjHwSm158iAviP1fS2mc2kuMc4Vf2k4M88hgN1reCUVGn5ufwxHmMEAZVXi82L2k6XLxNY,6HbdGdict6Egw8gwBRYmdgrMWt46qw1LtqkVk51D4sQx51XMDNEbsX6mcXZ1PjJJDy
```
