CGO_ENABLED ?= 1
BINARY ?= model-server

.PHONY: build test clean

build:
	CGO_ENABLED=$(CGO_ENABLED) go build -o $(BINARY) .

test:
	CGO_ENABLED=$(CGO_ENABLED) go test ./...

clean:
	rm -f $(BINARY)
