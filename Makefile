CURRENT_REVISION = $(shell git rev-parse --short HEAD)
BUILD_LDFLAGS = "-s -w -X github.com/akiym/akitools/cli.revision=$(CURRENT_REVISION)"
GOBIN ?= $(shell go env GOPATH)/bin

COMMANDS = \
	binary2png \
	command-wrapper \
	d \
	gadgets \
	gistwrapper \
	libc-offsets \
	o \
	rotn \
	tobin \
	tohex

.PHONY: build
build: cmd/command_wrapper/command-wrapper bin/akitools $(COMMANDS)

cmd/command_wrapper/command-wrapper: cmd/command_wrapper/_command-wrapper.c
	$(CC) -o $@ cmd/command_wrapper/_command-wrapper.c

.PHONY: bin/akitools
bin/akitools:
	go build -ldflags=$(BUILD_LDFLAGS) -o bin/akitools

.PHONY: $(COMMANDS)
$(COMMANDS):
	ln -sf akitools bin/$@

.PHONY: clean
clean:
	rm -f cmd/command_wrapper/command-wrapper
	rm -f bin/*

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
