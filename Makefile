VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS = -s -w -X github.com/maxtechera/admirarr/internal/ui.Version=$(VERSION)

.PHONY: build install clean test lint snapshot release

build:
	go build -ldflags "$(LDFLAGS)" -o admirarr-go .

install:
	go install -ldflags "$(LDFLAGS)" .

clean:
	rm -f admirarr-go
	rm -rf dist/

test:
	go test ./...

lint:
	golangci-lint run

snapshot:
	goreleaser release --snapshot --clean

release:
	goreleaser release --clean
