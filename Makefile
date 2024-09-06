.PHONY: default
default: build

.PHONY: build
build:
	go build -o bin/launcher-extras

.PHONY: test
test:
	go test ./...
