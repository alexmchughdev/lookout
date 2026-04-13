BINARY  := lookout
VERSION := 0.1.0
LDFLAGS := -ldflags "-X github.com/AlexMcHugh1/lookout/cmd.version=$(VERSION) -s -w"

.PHONY: build run tidy clean install cross

## Download deps and build
build: tidy
	go build $(LDFLAGS) -o $(BINARY) .

## Download deps
tidy:
	go mod tidy

## Install to GOPATH/bin
install: tidy
	go install $(LDFLAGS) .

## Run tests (unit only — integration needs a live app)
test:
	go test ./...

## Cross-compile for common targets
cross: tidy
	GOOS=linux  GOARCH=amd64 go build $(LDFLAGS) -o dist/lookout-linux-amd64 .
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/lookout-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/lookout-darwin-arm64 .
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/lookout-windows-amd64.exe .

## Remove build artifacts
clean:
	rm -f $(BINARY)
	rm -rf dist/

## Quick smoke test against a local app (set LOOKOUT_* env vars first)
smoke:
	./$(BINARY) run examples/demo.yaml \
		--url $(LOOKOUT_URL) \
		--email $(LOOKOUT_EMAIL) \
		--password $(LOOKOUT_PASSWORD) \
		--sections auth
