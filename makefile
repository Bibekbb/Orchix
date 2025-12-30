.PHONY: build clean test install lint

BINARY_NAME=orchix
VERSION=v0.1.0

build:
	@echo "Building ${BINARY_NAME} ${VERSION}..."
	@go build -ldflags="-X 'main.Version=${VERSION}'" -o ${BINARY_NAME} ./cmd/Orchix

clean:
	@echo "Cleaning..."
	@rm -f ${BINARY_NAME}
	@rm -rf dist/

test:
	@echo "Running tests..."
	@go test ./... -v

install: build
	@echo "Installing..."
	@sudo cp ${BINARY_NAME} /usr/local/bin/

lint:
	@echo "Linting..."
	@golangci-lint run

run:
	@go run ./cmd/Orchix/main.go --help