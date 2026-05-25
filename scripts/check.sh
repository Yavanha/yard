#!/usr/bin/env sh
set -eu

go run ./cmd/yard --help
go run ./cmd/yard config --project examples/web-app.yard.yml
go test ./...
