#!/bin/bash

COMMIT=$1

go get github.com/iotaledger/hive.go/constraints@$COMMIT
go get github.com/iotaledger/hive.go/stringify@$COMMIT
go get github.com/iotaledger/hive.go/serializer/v2@$COMMIT

go get github.com/iotaledger/hive.go/lo@$COMMIT

go get github.com/iotaledger/hive.go/ds@$COMMIT

go get github.com/iotaledger/hive.go/runtime@$COMMIT

go get github.com/iotaledger/hive.go/core@$COMMIT

go get github.com/iotaledger/hive.go/app@$COMMIT

go get github.com/iotaledger/hive.go/kvstore@$COMMIT

go get github.com/iotaledger/hive.go/objectstorage@$COMMIT
go get github.com/iotaledger/hive.go/autopeering@$COMMIT