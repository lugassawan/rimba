VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE    ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS  = -s -w \
	-X github.com/lugassawan/rimba/cmd.version=$(VERSION) \
	-X github.com/lugassawan/rimba/cmd.commit=$(COMMIT) \
	-X github.com/lugassawan/rimba/cmd.date=$(DATE)

.PHONY: build test test-short test-e2e test-coverage clean fmt lint bench hooks

build:
	go build -ldflags '$(LDFLAGS)' -o bin/rimba .

test:
	go test ./... -v -count=1

test-short:
	go test ./... -v -short -count=1

clean:
	rm -rf bin/ custom-gcl

fmt:
	go fmt ./...

test-e2e:
	go test ./tests/e2e/ -v -count=1 -timeout 120s

test-coverage:
	go test ./... -coverprofile=coverage.out -count=1
	go tool cover -func=coverage.out

lint: custom-gcl
	./custom-gcl run ./...

custom-gcl:
	golangci-lint custom

bench:
	go test -bench=. -benchmem -run=^$$ ./...

hooks:
	git config core.hooksPath .githooks
	@echo "Git hooks activated from .githooks/"
