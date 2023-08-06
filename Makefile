build:
	@go build -o bin/gohttp

run: build
	@./bin/gohttp

test:
	@go test -v ./...
	