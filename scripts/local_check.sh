#!/bin/sh
set -eu

go test ./...
go build -o /tmp/finchat-example ./cmd/finchat
