SHELL := /bin/bash

.PHONY:test
test:
	go test -coverprofile=coverage.txt -covermode=atomic ./...
	go vet ./...
	go install honnef.co/go/tools/cmd/staticcheck@latest
	staticcheck -checks all ./...
	go install github.com/kisielk/errcheck@latest
	errcheck -asserts ./...
	gofmt -e -l -d -s .
	go mod tidy

.PHONY:cover
cover: test
	go tool cover -func=coverage.txt

.PHONY:race
race:
	go test -race ./...

.PHONY:benchmark
benchmark:
	go test -bench=. -benchmem

.PHONY:benchmark-getzone
benchmark-getzone:
	go test -bench=BenchmarkGetZone -benchtime 30s -benchmem -cpuprofile cpuprofile.out -memprofile memprofile.out

.PHONY:benchmark-clientinit
benchmark-clientinit:
	go test -bench=BenchmarkClientInit -benchmem -cpuprofile cpuprofile.out -memprofile memprofile.out

.PHONY:cpuprof
cpuprof:
	go tool pprof -top cpuprofile.out | head -n 20

.PHONY:memprof
memprof:
	go tool pprof -top memprofile.out | head -n 20
