.PHONY: build test release clean run-relay

BINARY=p2p-drop

build:
	go build -ldflags="-s -w" -o $(BINARY) ./cmd/p2p-drop

test:
	go test -v ./...

release:
	./scripts/build-releases.sh v1.0.0

tunnel:
	./scripts/start-relay-tunnel.sh

run-relay:
	go run ./cmd/p2p-drop relay --port 8080

clean:
	rm -rf $(BINARY) dist/
