# Targets
.PHONY: all install run test clean lint

all: install

install:
	echo "Installing dependencies..." && \
    go mod tidy

run:
	echo "Running the application..." && \
	go run ./cmd/main.go

test:
	echo "Running tests..." && \
	go test ./... -v

clean:
	echo "Cleaning up..." && \
	go clean

lint:
	echo "Running linters..." && \
	golangci-lint run ./... && \
	gofmt -s -w .

