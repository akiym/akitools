CURRENT_REVISION = $(shell git rev-parse --short HEAD)
BUILD_LDFLAGS = "-s -w -X main.revision=$(CURRENT_REVISION)"

COMMANDS = \
	binary2png \
	ccwrap \
	cmdsbx \
	command-wrapper \
	d \
	docidx \
	gadgets \
	gistwrapper \
	git-branch-recent \
	git-sign \
	htmlmd \
	jwt \
	libc-offsets \
	noln \
	o \
	random_string \
	rotn \
	sbx-backup \
	shellcode \
	tobin \
	tohex \
	wag \
	wfind

.PHONY: all
all: build

.PHONY: build
build: bin $(COMMANDS)

.PHONY: build-all
build-all: bin-darwin bin-linux $(COMMANDS)

.PHONY: bin
bin: bin-darwin

.PHONY: bin-darwin
bin-darwin:
	GOOS=darwin GOARCH=arm64 go build -ldflags=$(BUILD_LDFLAGS) -o bin/darwin-arm64/akitools

.PHONY: bin-linux
bin-linux:
	GOOS=linux GOARCH=amd64 go build -ldflags=$(BUILD_LDFLAGS) -o bin/linux-amd64/akitools

.PHONY: $(COMMANDS)
$(COMMANDS):
	ln -sf akitools bin/darwin-arm64/$@
	ln -sf akitools bin/linux-amd64/$@

# -race needs cgo, i.e. a C compiler and libc headers on the machine.
.PHONY: test
test:
	go test -race ./...

# golangci-lint comes from PATH; the version is pinned in mise.toml.
.PHONY: fmt
fmt:
	golangci-lint fmt

# `run` does not check formatting, hence both steps.
.PHONY: check
check:
	golangci-lint fmt --diff
	golangci-lint run

.PHONY: clean
clean:
	rm -rf bin/*
