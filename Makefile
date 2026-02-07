VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE    ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS  = -s -w \
	-X github.com/lugassawan/rimba/cmd.version=$(VERSION) \
	-X github.com/lugassawan/rimba/cmd.commit=$(COMMIT) \
	-X github.com/lugassawan/rimba/cmd.date=$(DATE)

.PHONY: build test test-short clean lint

build:
	go build -ldflags '$(LDFLAGS)' -o bin/rimba .

test:
	go test ./... -v -count=1

test-short:
	go test ./... -v -short -count=1

clean:
	rm -rf bin/

lint:
	golangci-lint run ./...
