#!/bin/bash

COMMIT=$1

MODULES="ads app autopeering constraints core crypto ds kvstore lo logger objectstorage runtime serializer/v2 stringify"
for i in $MODULES
do
	go get -u github.com/iotaledger/hive.go/$i@$COMMIT

done

go mod tidy

pushd tools/integration-tests/tester
go mod tidy
popd
