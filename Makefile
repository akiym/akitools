CURRENT_REVISION = $(shell git rev-parse --short HEAD)
BUILD_LDFLAGS = "-s -w -X github.com/akiym/akitools/cli.revision=$(CURRENT_REVISION)"
GOBIN ?= $(shell go env GOPATH)/bin

COMMANDS = \
	binary2png \
	command-wrapper \
	d \
	gadgets \
	gistwrapper \
	git-branch-recent \
	libc-offsets \
	o \
	random_string \
	rotn \
	shellcode \
	tobin \
	tohex

.PHONY: build
build: bin $(COMMANDS)

.PHONY: bin
bin:
	GOOS=darwin GOARCH=arm64 go build -ldflags=$(BUILD_LDFLAGS) -o bin/darwin-arm64/akitools
	GOOS=darwin GOARCH=amd64 go build -ldflags=$(BUILD_LDFLAGS) -o bin/darwin-amd64/akitools
	GOOS=linux GOARCH=amd64 go build -ldflags=$(BUILD_LDFLAGS) -o bin/linux-amd64/akitools

.PHONY: $(COMMANDS)
$(COMMANDS):
	ln -sf akitools bin/darwin-arm64/$@
	ln -sf akitools bin/darwin-amd64/$@
	ln -sf akitools bin/linux-amd64/$@

.PHONY: clean
clean:
	rm -rf bin/*

.PHONY: install
install:
	go install -ldflags=$(BUILD_LDFLAGS)

.PHONY: test
test:
	go test -v -race ./...

.PHONY: lint
lint: $(GOBIN)/staticcheck
	go vet ./...
	staticcheck ./...

$(GOBIN)/staticcheck:
	go install honnef.co/go/tools/cmd/staticcheck@latest
