SHELL := /bin/bash

.PHONY:all
all: test

.PHONY:clean
clean:
	rm memprofile.out cpuprofile.out localtimezone.test c.out || true

.PHONY:install-test-deps
install-test-deps:
	go install honnef.co/go/tools/cmd/staticcheck@v0.4.0
	go install github.com/kisielk/errcheck@v1.6.3

.PHONY:test
test: install-test-deps unit
	go vet ./...
	staticcheck -checks all ./...
	errcheck -asserts ./...
	gofmt -e -l -d -s .
	go mod tidy

.PHONY:unit
unit:
	go test -coverprofile=c.out -covermode=atomic ./...

.PHONY:cover
cover: test
	go tool cover -func=c.out
	sed -i 's/github.com\/albertyw\/localtimezone\/v3\///g' c.out
	sed -i '/tzshapefilegen/d' c.out

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
