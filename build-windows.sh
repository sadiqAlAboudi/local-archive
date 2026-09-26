#!/bin/bash
set -e

echo "Building Local Archive Windows 64-bit binary (archive.exe)..."
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o archive.exe .

echo "Preparing installer payload..."
mkdir -p cmd/installer/payload
cp archive.exe cmd/installer/payload/archive.exe
cp static/favicon.ico cmd/installer/payload/favicon.ico

echo "Building Windows Installer (LocalArchive-Setup.exe)..."
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w -H windowsgui" -o LocalArchive-Setup.exe ./cmd/installer

echo "Done! Generated files:"
ls -lh archive.exe LocalArchive-Setup.exe
