#!/bin/bash
echo "copying assets into shared volume..."
rm -rf /assets/*
cp -rp /tmp/assets/* /assets
chmod 777 /assets/*
echo "assets:"
ls /assets
echo "running tests..."
go test ./tests/"${TEST_NAME}" -v -timeout 10m

# if running in CI we need to set right permissions on the Go folder (within container)
echo "CURRENT_UID ${CURRENT_UID}"
if [ ! -z ${CURRENT_UID} ]; then
  echo "setting permissions on Go folder... '${CURRENT_UID}'"
  chown -R "${CURRENT_UID}" /go /root/.cache/go-build
fi
