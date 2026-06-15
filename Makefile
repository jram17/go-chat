.PHONY: build server client certs test clean

build: server client

server:
	go build -o bin/server ./cmd/server

client:
	go build -o bin/client ./cmd/client

certs:
	go run ./cmd/gen-cert

test:
	go test $$(go list ./... | grep -v test-dummy)

clean:
	rm -rf bin/
