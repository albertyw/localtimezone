SHELL := /bin/bash

.PHONY:test
test:
	go test -coverprofile=coverage.txt -covermode=atomic ./...
	go vet ./...
	go install honnef.co/go/tools/cmd/staticcheck@latest
	staticcheck -checks all ./...
	gofmt -e -l -d -s .
	go mod tidy

.PHONY:race
race:
	go test -race ./...
