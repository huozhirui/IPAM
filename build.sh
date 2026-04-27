#!/bin/bash
set -e

cd "$(dirname "$0")"

echo "==> Building frontend..."
cd web && npm install && npm run build
cd ..

echo "==> Copying frontend assets..."
rm -rf static/dist
cp -r web/dist static/dist

echo "==> Building backend..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o network-plan ./cmd/main.go

echo "==> Done: ./network-plan"
