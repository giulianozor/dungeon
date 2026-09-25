.PHONY: all build run test clean lint fmt tidy

BINARY=server

all: build

build:
	go build -o $(BINARY) ./cmd/server

run: build
	./$(BINARY) ${ARGS}

test:
	go test ./...

clean:
	rm -f $(BINARY)
	go clean

lint:
	@command -v staticcheck >/dev/null 2>&1 || go install honnef.co/go/tools/cmd/staticcheck@latest
	staticcheck ./...

fmt:
	go fmt ./...

tidy:
	go mod tidy
