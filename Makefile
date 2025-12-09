# Makefile for SnapShift

.PHONY: build clean install test fmt vet help

# Binary name
BINARY_NAME=snapshift

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt
GOVET=$(GOCMD) vet

all: test build

## build: Build the binary
build:
	$(GOBUILD) -o $(BINARY_NAME) -v

## clean: Clean build files
clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)

## install: Install the binary to $GOPATH/bin
install:
	$(GOCMD) install

## test: Run tests
test:
	$(GOTEST) -v ./...

## fmt: Format code
fmt:
	$(GOFMT) ./...

## vet: Run go vet
vet:
	$(GOVET) ./...

## deps: Download dependencies
deps:
	$(GOMOD) download
	$(GOMOD) tidy

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' |  sed -e 's/^/ /'
