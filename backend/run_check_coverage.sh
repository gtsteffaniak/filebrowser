#!/bin/bash
go test -race -v -coverpkg=./... -coverprofile=coverage.cov ./...
go tool cover -html=coverage.cov
