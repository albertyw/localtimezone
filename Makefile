SHELL := /bin/bash

.PHONY:test
test:
	go test -coverprofile=coverage.txt -covermode=atomic ./...
	go vet ./...
	go install honnef.co/go/tools/cmd/staticcheck@latest
	staticcheck -checks all ./...
	gofmt -e -l -d -s .
	go mod tidy

.PHONY:cover
cover: test
	go tool cover -func=coverage.txt

.PHONY:benchmark
benchmark:
	go test -bench=.

.PHONY:race
race:
	go test -race ./...
