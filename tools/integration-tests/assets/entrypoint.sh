#!/bin/bash
echo "copying assets into shared volume..."
rm -rf /assets/*
cp -rp /tmp/assets/* /assets
chmod 777 /assets/*
mkdir /assets/dynamic_snapshots
echo "assets:"
ls /assets
if [ "x$DEBUG" = "x1" ]; then
  echo "install debugging tools..."
  go install github.com/go-delve/delve/cmd/dlv@master
  echo "debugging tests..."
  dlv test --headless --listen ':40000' --accept-multiclient --api-version 2 ./tests/${TEST_NAME}
else
  echo "running tests..."
  go test ./tests/"${TEST_NAME}" -v -timeout 30m -count=1
fi