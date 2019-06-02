#!/bin/bash

echo "Building centralized PRNG..."
cd cprng
go build -o prng.exe
echo "Starting centralized PRNG"
./prng.exe -interval=2 -port=10000