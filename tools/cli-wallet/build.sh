#!/bin/bash

echo "Building executables..."

GOOS=windows GOARCH=amd64 go build -o cli-wallet_Windows_x86_64.exe
echo "Windows version created"
GOOS=linux GOARCH=amd64 go build -o cli-wallet_Linux_x86_64
echo "Linux version created"
GOOS=darwin GOARCH=amd64 go build -o cli-wallet_macOS_x86_64
echo "MAC OSX version created"

echo "All done!"