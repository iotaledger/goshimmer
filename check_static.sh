#!/bin/bash
# This script uses ldd utility to verify that result goshimmer binary is statically linked

ldd /go/bin/goshimmer &> /dev/null
if [ $? -ne 0 ]; then
  exit 0
else
  echo "/go/bin/goshimmer must be a statically linked binary"
  exit 1
fi
